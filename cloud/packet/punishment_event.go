package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type PunishmentEvent struct {
	PunishmentID string // 1-5 bytes + len(PunishmentID) bytes
	EffectiveAt  int64  // 8 bytes
}

func (*PunishmentEvent) ID() uint32 {
	return IDPunishmentEvent
}

func (pk *PunishmentEvent) Marshal(io protocol.IO, cloudProto uint32) {
	io.String(&pk.PunishmentID)
	io.Int64(&pk.EffectiveAt)
}
