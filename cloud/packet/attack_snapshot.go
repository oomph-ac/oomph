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
	HotBarSlot  int32      // 4 bytes
	EntityRID   uint64     // 8 bytes
	ReportedPos mgl32.Vec3 // 12 bytes
	ClickedPos  mgl32.Vec3 // 12 bytes
}

func (*AttackSnapshot) ID() uint32 {
	return IDAttackSnapshot
}

func (pk *AttackSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Int32(&pk.HotBarSlot)
	io.Uint64(&pk.EntityRID)
	io.Vec3(&pk.ReportedPos)
	io.Vec3(&pk.ClickedPos)
}
