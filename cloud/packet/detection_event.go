package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	DetectionEventFlagged byte = iota
	DetectionEventPunishmentQueued
	DetectionEventPunishmentIssued
)

func init() {
	Register(IDDetectionEvent, func() Packet { return &DetectionEvent{} })
}

type DetectionEvent struct {
	EventType   byte
	Violations  float32
	DetectionID string
}

func (pk *DetectionEvent) ID() uint32 {
	return IDDetectionEvent
}

func (pk *DetectionEvent) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.EventType)
	io.String(&pk.DetectionID)
	if pk.EventType == DetectionEventFlagged {
		io.Float32(&pk.Violations)
	}
}
