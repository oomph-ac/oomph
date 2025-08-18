package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	DetectionEventFlagged byte = iota
	DetectionEventPunishmentIssued
)

func init() {
	Register(IDDetectionEvent, func() Packet { return &DetectionEvent{} })
}

type DetectionEvent struct {
	EventType byte

	Violations       float32
	DetectionType    string
	DetectionSubType string

	PunishmentID          string
	PunishmentEffectiveAt int64
}

func (pk *DetectionEvent) ID() uint32 {
	return IDDetectionEvent
}

func (pk *DetectionEvent) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.EventType)
	if pk.EventType == DetectionEventFlagged {
		io.String(&pk.DetectionType)
		io.String(&pk.DetectionSubType)
		io.Float32(&pk.Violations)
	} else if pk.EventType == DetectionEventPunishmentIssued {
		io.String(&pk.PunishmentID)
		io.Int64(&pk.PunishmentEffectiveAt)
	}
}
