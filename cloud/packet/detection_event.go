package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDDetectionEvent, func() Packet { return &DetectionEvent{} })
}

type DetectionEvent struct {
	// PlayerIdentifier is either the XUID of the player if the proxy has online mode enabled, or the client-side
	// generated UUID of the player if the proxy wants to accept offline-mode players.
	PlayerIdentifier string // 1-5 bytes + len(PlayerIdentifier)

	Violations float32 // 4 bytes
	Type       string  // 1-5 bytes + len(Type) bytes
	SubType    string  // 1-5 bytes + len(SubType) bytes
}

func (pk *DetectionEvent) ID() uint32 {
	return IDDetectionEvent
}

func (pk *DetectionEvent) Marshal(io protocol.IO, cloudProto uint32) {
	io.String(&pk.PlayerIdentifier)
	io.String(&pk.Type)
	io.String(&pk.SubType)
	io.Float32(&pk.Violations)
}
