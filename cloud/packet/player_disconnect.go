package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDPlayerDisconnect, func() Packet {
		return &PlayerDisconnect{}
	})
}

// PlayerDisconnect can be sent by both the connected proxy and the cloud server. If sent by the cloud server, the proxy
// should disconnect the player with the given message. If sent by the connected proxy, the cloud server should save relevant
// data before removing the session.
type PlayerDisconnect struct {
	Identifier string // 1-5 bytes + len(Identifier)
	Message    string // 1-5 bytes + len(Message) bytes
}

func (*PlayerDisconnect) ID() uint32 {
	return IDPlayerDisconnect
}

func (pk *PlayerDisconnect) Marshal(io protocol.IO, cloudProto uint32) {
	io.String(&pk.Identifier)
	io.String(&pk.Message)
}
