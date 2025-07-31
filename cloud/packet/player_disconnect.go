package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDPlayerDisconnect, func() Packet {
		return &PlayerDisconnect{}
	})
}

// PlayerDisconnect represents a packet sent by Oomph's cloud when it wants to disconnect a player from the connected proxy.
type PlayerDisconnect struct {
	Message string
}

func (*PlayerDisconnect) ID() uint32 {
	return IDPlayerDisconnect
}

func (pk *PlayerDisconnect) Marshal(io protocol.IO, cloudProto uint32) {
	io.String(&pk.Message)
}
