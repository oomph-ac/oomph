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

const (
	AttackSnapshotFlagIsInital = 1 << iota
	AttackSnapshotFlagUpdatedHotBarSlot
	AttackSnapshotFlagUpdatedEntityRID
	AttackSnapshotFlagUpdatedReportedPos
	AttackSnapshotFlagUpdatedClickedPos
)

type AttackSnapshot struct {
	Flags uint8 // 1 byte

	HotBarSlot  int32      // 0-5 bytes
	EntityRID   uint64     // 0-9 bytes
	ReportedPos mgl32.Vec3 // 0-12 bytes
	ClickedPos  mgl32.Vec3 // 0-12 bytes
}

func (*AttackSnapshot) ID() uint32 {
	return IDAttackSnapshot
}

func (pk *AttackSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.Flags)
	if pk.CheckFlag(AttackSnapshotFlagIsInital) {
		io.Varint32(&pk.HotBarSlot)
		io.Varuint64(&pk.EntityRID)
		io.Vec3(&pk.ReportedPos)
		io.Vec3(&pk.ClickedPos)
		return
	}

	if pk.CheckFlag(AttackSnapshotFlagUpdatedHotBarSlot) {
		io.Varint32(&pk.HotBarSlot)
	}
	if pk.CheckFlag(AttackSnapshotFlagUpdatedEntityRID) {
		io.Varuint64(&pk.EntityRID)
	}
	if pk.CheckFlag(AttackSnapshotFlagUpdatedReportedPos) {
		io.Vec3(&pk.ReportedPos)
	}
	if pk.CheckFlag(AttackSnapshotFlagUpdatedClickedPos) {
		io.Vec3(&pk.ClickedPos)
	}
}

func (pk *AttackSnapshot) SetHotBarSlot(s int32) {
	pk.HotBarSlot = s
	pk.Flags |= AttackSnapshotFlagUpdatedHotBarSlot
}

func (pk *AttackSnapshot) SetEntityRID(rid uint64) {
	pk.EntityRID = rid
	pk.Flags |= AttackSnapshotFlagUpdatedEntityRID
}

func (pk *AttackSnapshot) SetReportedPos(pos mgl32.Vec3) {
	pk.ReportedPos = pos
	pk.Flags |= AttackSnapshotFlagUpdatedReportedPos
}

func (pk *AttackSnapshot) SetClickedPos(pos mgl32.Vec3) {
	pk.ClickedPos = pos
	pk.Flags |= AttackSnapshotFlagUpdatedClickedPos
}

func (pk *AttackSnapshot) CheckFlag(flag uint8) bool {
	return pk.Flags&flag == flag
}
