package packet

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func init() {
	Register(IDAddPlayerRequest, func() Packet {
		return &AddPlayerRequest{}
	})
}

type AddPlayerRequest struct {
	// PlayerIdentifier is either the XUID of the player if the proxy has online mode enabled, or the client-side
	// generated UUID of the player if the proxy wants to accept offline-mode players.
	// This is included in the packet as in the case for some reason where the client data or identity data are not able
	// to be parsed, a response can still be sent back to the proxy.
	PlayerIdentifier string // 1-5 bytes + len(PlayerIdentifier) bytes

	Protocol     int32                     // 1-5 bytes
	Address      string                    // 1-5 bytes + len(Address) bytes
	ClientData   []byte                    // 1-5 bytes + len(ClientData) bytes
	IdentityData protocol.Optional[[]byte] // 1 byte + 1-5 bytes + len(IdentityData) bytes
}

func (*AddPlayerRequest) ID() uint32 {
	return IDAddPlayerRequest
}

func (pk *AddPlayerRequest) Marshal(io protocol.IO, _ uint32) {
	io.Varint32(&pk.Protocol)
	io.String(&pk.Address)
	io.ByteSlice(&pk.ClientData)
	protocol.OptionalFunc(io, &pk.IdentityData, io.ByteSlice)
}
