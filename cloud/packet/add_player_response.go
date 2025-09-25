package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	AddPlayerResponseStatusOK uint8 = iota
	AddPlayerResponseStatusAlreadyConnected
	AddPlayerResponseStatusError
	AddPlayerResponseStatusErrorNeedPlayerDisconnect
)

func init() {
	Register(IDAddPlayerResponse, func() Packet {
		return &AddPlayerResponse{}
	})
}

type AddPlayerResponse struct {
	Status  uint8  // 1 byte
	CloudID uint64 // 8 bytes

	// PlayerIdentifier is either the PlayerIdentifier of the player if the proxy has online mode enabled, or the client-side
	// generated UUID of the player if the proxy wants to accept offline-mode players.
	PlayerIdentifier string // 1-5 bytes + len(XUID) bytes

	Message string // 1-5 bytes + len(Message) bytes
}

func (*AddPlayerResponse) ID() uint32 {
	return IDAddPlayerResponse
}

func (pk *AddPlayerResponse) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.Status)
	io.Varuint64(&pk.CloudID)
	io.String(&pk.PlayerIdentifier)
	if pk.Status != AddPlayerResponseStatusOK {
		io.String(&pk.Message)
	}
}
