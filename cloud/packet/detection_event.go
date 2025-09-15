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
	XUID                  string  // 1-5 bytes + len(XUID)
	EventType             byte    // 1 byte
	Violations            float32 // 4 bytes
	DetectionType         string  // 1-5 bytes + len(DetectionType) bytes
	DetectionSubType      string  // 1-5 bytes + len(DetectionSubType) bytes
	PunishmentID          string  // 1-5 bytes + len(PunishmentID) bytes
	PunishmentEffectiveAt int64   // 8 bytes
}

func (pk *DetectionEvent) ID() uint32 {
	return IDDetectionEvent
}

func (pk *DetectionEvent) Marshal(io protocol.IO, cloudProto uint32) {
	io.String(&pk.XUID)
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
