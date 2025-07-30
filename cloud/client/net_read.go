package client

import (
	"bytes"
	"fmt"
	"io"

	"github.com/golang/snappy"
	"github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/internal"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func (c *Client) ReadPacket() (packet.Packet, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("client is not connected")
	}

	select {
	case <-c.done:
		return nil, fmt.Errorf("client is closed")
	case pk := <-c.rPackets:
		return pk, nil
	}
}

func (c *Client) readLoop() {
	header := make([]byte, packet.NetworkHeaderSize)
	for {
		select {
		case <-c.done:
			return
		default:
			conn := c.conn
			if conn == nil {
				c.log.Error("connection is nil, cannot read packet")
				return
			}

			// Read the packet header
			if _, err := io.ReadFull(conn, header); err != nil {
				c.log.Error("failed to read packet header", "err", err)
				c.Close()
				return
			}
			headerGb := float64(len(header)) * ByteToGBMultiplier
			_ = gbIn.Add(headerGb)
			_ = gbProcIn.Add(headerGb)

			if header[4] != 0 && header[4] != 1 {
				c.log.Error("invalid packet header: compression flag must be 0 or 1", "flag", header[4])
				c.Close()
				return
			}

			// Parse header: first 4 bytes are length, 5th byte is compression flag
			payloadLen := uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16 | uint32(header[3])<<24
			compressed := header[4] == 1
			if payloadLen > packet.MaxBatchSize {
				c.log.Error("packet payload exceeds maximum size", "size", payloadLen)
				c.Close()
				return
			}

			// Read the packet payload
			payload := make([]byte, payloadLen)
			if _, err := io.ReadFull(conn, payload); err != nil {
				c.log.Error("failed to read packet payload", "err", err)
				c.Close()
				return
			}
			_ = gbIn.Add(float64(len(payload)) * ByteToGBMultiplier)

			// Decompress if needed
			var data []byte
			if compressed {
				var err error
				data, err = snappy.Decode(nil, payload)
				if err != nil {
					c.log.Error("failed to decompress packet", "err", err)
					return
				}
			} else {
				data = payload
			}
			_ = gbProcIn.Add(float64(len(data)) * ByteToGBMultiplier)

			// Parse the batch data
			if err := c.parseBatch(data); err != nil {
				c.log.Error("failed to parse batch", "err", err)
				c.Close()
				return
			}
		}
	}
}

func (c *Client) parseBatch(data []byte) error {
	batchBuf := bytes.NewBuffer(data)
	reader := protocol.NewReader(batchBuf, 0, false)
	for batchBuf.Len() > 0 {
		var pkId, pkLen uint32
		reader.Varuint32(&pkId)
		reader.Varuint32(&pkLen)

		if batchBuf.Len() < int(pkLen) {
			return fmt.Errorf("insufficient data for packet %d: expected %d bytes, have %d", pkId, pkLen, batchBuf.Len())
		}

		pkBuf := bytes.NewBuffer(batchBuf.Next(int(pkLen)))
		pkReader := protocol.NewReader(pkBuf, 0, false)

		pk, ok := packet.Get(pkId)
		if !ok {
			c.log.Warn("unknown packet ID", "id", pkId)
			continue
		}
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
