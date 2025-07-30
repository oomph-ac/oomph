package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func init() {
	Register(IDAttackSnapshot, func() Packet {
		return &AttackSnapshot{}
	})
}

type AttackSnapshot struct {
	IsInital bool // 1 byte

	HotBarSlot  protocol.Optional[int32]      // 1-5 bytes
	EntityRID   protocol.Optional[uint64]     // 1-9 bytes
	ReportedPos protocol.Optional[mgl32.Vec3] // 1-13 bytes
	ClickedPos  protocol.Optional[mgl32.Vec3] // 1-13 bytes
}

func (*AttackSnapshot) ID() uint32 {
	return IDAttackSnapshot
}

func (pk *AttackSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Bool(&pk.IsInital)
	protocol.OptionalFunc(io, &pk.HotBarSlot, io.Varint32)
	protocol.OptionalFunc(io, &pk.EntityRID, io.Uint64)
	protocol.OptionalFunc(io, &pk.ReportedPos, io.Vec3)
	protocol.OptionalFunc(io, &pk.ClickedPos, io.Vec3)
}
