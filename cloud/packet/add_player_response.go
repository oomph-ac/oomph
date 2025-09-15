package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	AddPlayerResponseStatusOK uint8 = iota
	AddPlayerResponseStatusOKWithWarning
	AddPlayerResponseStatusAlreadyConnected
	AddPlayerResponseStatusError
)

func init() {
	Register(IDAddPlayerResponse, func() Packet {
		return &AddPlayerResponse{}
	})
}

type AddPlayerResponse struct {
	Status  uint8  // 1 byte
	XUID    string // 1-5 bytes + len(XUID) bytes
	Message string // 1-5 bytes + len(Message) bytes
}

func (*AddPlayerResponse) ID() uint32 {
	return IDAddPlayerResponse
}

func (pk *AddPlayerResponse) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.Status)
	if pk.Status != AddPlayerResponseStatusOK {
		io.String(&pk.Message)
	}
}
