package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	InitConnectionStatusOK uint8 = iota
	InitConnectionStatusOKWithWarning
	InitConnectionStatusError
	InitConnectionStatusUnauthorized
)

func init() {
	Register(IDInitConnectionResponse, func() Packet {
		return &InitConnectionResponse{}
	})
}

// InitConnectionResponse is a packet sent back from the server after receiving a inital connection request.
// It is meant to inform the client about whether their connection was accepted or not.
type InitConnectionResponse struct {
	Status  uint8  // 1 byte
	Message string // 1-5 bytes + len(Message) bytes
}

func (*InitConnectionResponse) ID() uint32 {
	return IDInitConnectionResponse
}

func (pk *InitConnectionResponse) Marshal(io protocol.IO, _ uint32) {
	io.Uint8(&pk.Status)
	if pk.Status != InitConnectionStatusOK {
		io.String(&pk.Message)
	}
}
