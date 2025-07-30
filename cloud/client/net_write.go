package client

import (
	"fmt"
	"time"

	"github.com/golang/snappy"
	"github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/internal"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func (c *Client) WritePacket(pk packet.Packet) error {
	if c.conn == nil {
		return fmt.Errorf("cannot write packet to disconnected client")
	}

	select {
	case <-c.done:
		return fmt.Errorf("client is closed")
	case c.wPackets <- pk:
		return nil
	}
}

func (c *Client) flush() error {
	if len(c.batched) == 0 {
		return nil
	}

	buf := internal.NewBatchBuf()
	defer internal.PutBatchBuf(buf)

	batchWriter := protocol.NewWriter(buf, 0)
	for _, pk := range c.batched {
		pkBuf := internal.NewPacketBuf()
		pkWriter := protocol.NewWriter(pkBuf, 0)
		pk.Marshal(pkWriter, packet.CurrentProtocol)
		pkId, pkLen, pkBytes := pk.ID(), uint32(pkBuf.Len()), pkBuf.Bytes()
		batchWriter.Varuint32(&pkId)
		batchWriter.Varuint32(&pkLen)
		batchWriter.Bytes(&pkBytes)
		internal.PutPacketBuf(pkBuf)
	}

	batchLen := uint32(buf.Len())
	useCompression := batchLen > c.connInfo.CmpThreshold
	header := make([]byte, packet.NetworkHeaderSize)
	gbOut += float64(len(header)) * ByteToGBMultiplier
	gbProcOut += float64(len(header)) * ByteToGBMultiplier
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
		gbProcOut += float64(len(buf.Bytes())) * ByteToGBMultiplier
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
	gbProcOut += float64(len(compressed)) * ByteToGBMultiplier
	return nil
}

func (c *Client) networkWrite(b []byte) error {
	const rewriteAttempts = 10
	remaining := len(b)
	conn := c.conn
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
	t := time.NewTicker(c.connInfo.FlushRate)
	defer t.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			if err := c.flush(); err != nil {
				c.log.Error("failed to flush packets", "err", err)
				return
			}
			c.batched = c.batched[:0]
		case pk := <-c.wPackets:
			c.batched = append(c.batched, pk)
		}
	}
}
