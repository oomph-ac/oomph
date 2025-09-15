package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDHeartbeat, func() Packet {
		return &Heartbeat{}
	})
}

// Heartbeat is a packet that is sent periodically to indicate that the remote connection is still active.
type Heartbeat struct {
	Timestamp int64 // 1-9 bytes
}

func (*Heartbeat) ID() uint32 {
	return IDHeartbeat
}

func (pk *Heartbeat) Marshal(io protocol.IO, cloudProto uint32) {
	io.Varint64(&pk.Timestamp)
}
