package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDCustomMessage, func() Packet {
		return &CustomMessage{}
	})
}

type CustomMessage struct {
	CloudID uint64 // 1-9 bytes
	Type    uint64 // 1-9 bytes
	Message []byte // 1-5 bytes + len(Message) bytes
}

func (*CustomMessage) ID() uint32 {
	return IDCustomMessage
}

func (pk *CustomMessage) Marshal(io protocol.IO, cloudProto uint32) {
	io.Varuint64(&pk.CloudID)
	io.Varuint64(&pk.Type)
	io.ByteSlice(&pk.Message)
}
