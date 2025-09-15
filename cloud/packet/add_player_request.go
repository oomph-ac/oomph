package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDAddPlayerRequest, func() Packet {
		return &AddPlayerRequest{}
	})
}

type AddPlayerRequest struct {
	Protocol     uint32 // 1-5 bytes
	Address      string // 1-5 bytes + len(Address) bytes
	ClientData   []byte // 1-5 bytes + len(ClientData) bytes
	IdentityData []byte // 1-5 bytes + len(IdentityData) bytes
}

func (*AddPlayerRequest) ID() uint32 {
	return IDAddPlayerRequest
}

func (pk *AddPlayerRequest) Marshal(io protocol.IO, _ uint32) {
	io.Varuint32(&pk.Protocol)
	io.String(&pk.Address)
	io.ByteSlice(&pk.ClientData)
	io.ByteSlice(&pk.IdentityData)
}
