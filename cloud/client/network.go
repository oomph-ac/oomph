package client

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/snappy"
	"github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/internal"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func (c *Client) WritePacket(pk packet.Packet) error {
	if c.Conn() == nil {
		// We don't have to return an error - since we're going to wait until the connection is re-established.
		return nil
	}

	select {
	case <-c.done:
		return fmt.Errorf("client is closed")
	case c.wPackets <- pk:
		return nil
	}
}

func (c *Client) ReadPacket() (packet.Packet, error) {
	select {
	case <-c.done:
		return nil, fmt.Errorf("client is closed")
	case pk := <-c.rPackets:
		return pk, nil
	}
}

func (c *Client) closeConn() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	// We check against connSetAt just in case the connection has been set, but the previous caller of client.Conn may have had nil returned
	// right before the connection was re-established.
	if c.conn != nil && time.Since(c.connSetAt) >= 15*time.Second {
		_ = c.conn.Close()
		c.conn = nil // Set connection to nil to trigger reconnection.
	}
}

func (c *Client) flush() error {
	if len(c.batched) == 0 {
		return nil
	}

	buf := internal.NewBatchBuf()
	defer internal.PutBatchBuf(buf)

	batchWriter := protocol.NewWriter(buf, 0)
	pkWriter := protocol.NewWriter(nil, 0)
	for _, pk := range c.batched {
		pkBuf := internal.NewPacketBuf()
		internal.ModifyWriterOutput(pkWriter, pkBuf)
		pk.Marshal(pkWriter, packet.CurrentProtocol)
		pkId, pkLen, pkBytes := pk.ID(), uint32(pkBuf.Len()), pkBuf.Bytes()
		batchWriter.Varuint32(&pkId)
		batchWriter.Varuint32(&pkLen)
		batchWriter.Bytes(&pkBytes)
		internal.PutPacketBuf(pkBuf)
	}

	batchLen := uint32(buf.Len())
	useCompression := batchLen >= c.cmpThreshold
	header := make([]byte, packet.NetworkHeaderSize)
	if !useCompression {
		header[0] = byte(batchLen)
		header[1] = byte(batchLen >> 8)
		header[2] = byte(batchLen >> 16)
		header[3] = byte(batchLen >> 24)
		header[4] = 0 // No compression

		if err := c.networkWrite(header); err != nil {
			return fmt.Errorf("failed to write packet header: %v", err)
		} else if err := c.networkWrite(buf.Bytes()); err != nil {
			return fmt.Errorf("failed to write packet data: %v", err)
		}
		return nil
	}

	compressed := snappy.Encode(nil, buf.Bytes())
	header[0] = byte(len(compressed))
	header[1] = byte(len(compressed) >> 8)
	header[2] = byte(len(compressed) >> 16)
	header[3] = byte(len(compressed) >> 24)
	header[4] = 1 // Compression used

	if err := c.networkWrite(header); err != nil {
		return fmt.Errorf("failed to write compressed packet header: %v", err)
	}
	if err := c.networkWrite(compressed); err != nil {
		return fmt.Errorf("failed to write compressed packet data: %v", err)
	}
	return nil
}

func (c *Client) networkWrite(b []byte) error {
	const rewriteAttempts = 10
	remaining := len(b)
	conn := c.Conn()
	if conn == nil {
		return fmt.Errorf("networkWrite: client connection is nil")
	}

	for range rewriteAttempts {
		n, err := conn.Write(b)
		if err != nil {
			return err
		}
		remaining -= n
		if remaining <= 0 {
			return nil
		}
		b = b[n:]
	}
	return fmt.Errorf("networkWrite: %d bytes still remaining after %d write attempts", remaining, rewriteAttempts)
}

func (c *Client) writeLoop() {
	t := time.NewTicker(c.flushRate)
	defer t.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			if err := c.flush(); err != nil && c.Conn() != nil {
				c.log.Error("failed to flush packets", "err", err)
				c.closeConn()
			}
			c.batched = c.batched[:0]
		case pk := <-c.wPackets:
			c.batched = append(c.batched, pk)
		}
	}
}

func (c *Client) readLoop() {
	header := make([]byte, packet.NetworkHeaderSize)
	for {
		select {
		case <-c.done:
			return
		default:
			conn := c.Conn()
			if conn == nil {
				// Wait for a re-connection to be established.
				time.Sleep(5 * time.Second)
				continue
			}

			// Read the packet header
			if _, err := io.ReadFull(conn, header); err != nil {
				c.log.Error("failed to read packet header", "err", err)
				c.closeConn()
				continue
			}

			if header[4] != 0 && header[4] != 1 {
				c.log.Error("invalid packet header: compression flag must be 0 or 1", "flag", header[4])
				c.closeConn()
				continue
			}

			// Parse header: first 4 bytes are length, 5th byte is compression flag
			payloadLen := uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16 | uint32(header[3])<<24
			compressed := header[4] == 1
			if payloadLen == 0 {
				continue
			}

			// Read the packet payload
			payload := make([]byte, payloadLen)
			if _, err := io.ReadFull(c.conn, payload); err != nil {
				c.log.Error("failed to read packet payload", "err", err)
				return
			}

			// Decompress if needed
			var data []byte
			if compressed {
				var err error
				data, err = snappy.Decode(nil, payload)
				if err != nil {
					c.log.Error("failed to decompress packet", "err", err)
					c.closeConn()
					continue
				}
			} else {
				data = payload
			}

			// Parse the batch data
			if err := c.parseBatch(data); err != nil {
				c.log.Error("failed to parse batch", "err", err)
				c.closeConn()
				continue
			}
		}
	}
}

func (c *Client) parseBatch(data []byte) error {
	buf := internal.NewBatchBuf()
	defer internal.PutBatchBuf(buf)
	buf.Write(data)

	reader := protocol.NewReader(buf, 0, false)

	for buf.Len() > 0 {
		var pkId, pkLen uint32
		reader.Varuint32(&pkId)
		reader.Varuint32(&pkLen)

		if buf.Len() < int(pkLen) {
			return fmt.Errorf("insufficient data for packet %d: expected %d bytes, have %d", pkId, pkLen, buf.Len())
		}

		pkData := make([]byte, pkLen)
		reader.Bytes(&pkData)

		pk, ok := packet.Get(pkId)
		if !ok {
			c.log.Warn("unknown packet ID", "id", pkId)
			continue
		}

		pkBuf := internal.NewPacketBuf()
		pkBuf.Write(pkData)
		pkReader := protocol.NewReader(pkBuf, 0, false)
		pk.Marshal(pkReader, packet.CurrentProtocol)
		internal.PutPacketBuf(pkBuf)

		select {
		case <-c.done:
			return nil
		case c.rPackets <- pk:
			// OK.
		}
	}

	return nil
}
