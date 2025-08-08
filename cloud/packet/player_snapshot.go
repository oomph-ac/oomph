package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	gtpacket "github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	PlayerSnapshotFlagInCorrectiveState = 1 << iota
	PlayerSnapshotFlagHasTeleport
	PlayerSnapshotFlagHasKnockback
	PlayerSnapshotFlagUpdatedPosition
	PlayerSnapshotFlagUpdatedClientPosition
	PlayerSnapshotFlagUpdatedVelocity
	PlayerSnapshotFlagUpdatedClientVelocity
	PlayerSnapshotFlagUpdatedMovement
	PlayerSnapshotFlagUpdatedClientMovement
	PlayerSnapshotFlagUpdatedRotation
)

func init() {
	Register(IDPlayerSnapshot, func() Packet {
		return &PlayerSnapshot{}
	})
}

type PlayerSnapshot struct {
	SnapshotFlags uint16          // 2 bytes
	CInputFlags   protocol.Bitset // 9 bytes (?)

	Pos, CPos mgl32.Vec3 // 0-24 bytes
	Vel, CVel mgl32.Vec3 // 0-24 bytes
	Mov, CMov mgl32.Vec3 // 0-24 bytes
	CRot      mgl32.Vec3 // 0-12 bytes

	Timestamp  int64  // 8 bytes
	CInputTick int64  // 8 bytes
	CSimTick   uint64 // 8 bytes

}

func (*PlayerSnapshot) ID() uint32 {
	return IDPlayerSnapshot
}

func (pk *PlayerSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint16(&pk.SnapshotFlags)
	io.Bitset(&pk.CInputFlags, gtpacket.PlayerAuthInputBitsetSize)

	if pk.CheckFlag(PlayerSnapshotFlagUpdatedPosition) {
		io.Vec3(&pk.Pos)
	}
	if pk.CheckFlag(PlayerSnapshotFlagUpdatedClientPosition) {
		io.Vec3(&pk.CPos)
	}
	if pk.CheckFlag(PlayerSnapshotFlagUpdatedVelocity) {
		io.Vec3(&pk.Vel)
	}
	if pk.CheckFlag(PlayerSnapshotFlagUpdatedClientVelocity) {
		io.Vec3(&pk.CVel)
	}
	if pk.CheckFlag(PlayerSnapshotFlagUpdatedMovement) {
		io.Vec3(&pk.Mov)
	}
	if pk.CheckFlag(PlayerSnapshotFlagUpdatedClientMovement) {
		io.Vec3(&pk.CMov)
	}
	if pk.CheckFlag(PlayerSnapshotFlagUpdatedRotation) {
		io.Vec3(&pk.CRot)
	}

	io.Varint64(&pk.Timestamp)
	io.Varint64(&pk.CInputTick)
	io.Varuint64(&pk.CSimTick)
}

func (pk *PlayerSnapshot) SetPos(pos mgl32.Vec3) {
	pk.Pos = pos
	pk.SnapshotFlags |= PlayerSnapshotFlagUpdatedPosition
}

func (pk *PlayerSnapshot) SetCPos(pos mgl32.Vec3) {
	pk.CPos = pos
	pk.SnapshotFlags |= PlayerSnapshotFlagUpdatedClientPosition
}

func (pk *PlayerSnapshot) SetVel(vel mgl32.Vec3) {
	pk.Vel = vel
	pk.SnapshotFlags |= PlayerSnapshotFlagUpdatedVelocity
}

func (pk *PlayerSnapshot) SetCVel(vel mgl32.Vec3) {
	pk.CVel = vel
	pk.SnapshotFlags |= PlayerSnapshotFlagUpdatedClientVelocity
}

func (pk *PlayerSnapshot) SetMov(mov mgl32.Vec3) {
	pk.Mov = mov
	pk.SnapshotFlags |= PlayerSnapshotFlagUpdatedMovement
}

func (pk *PlayerSnapshot) SetCMov(mov mgl32.Vec3) {
	pk.CMov = mov
	pk.SnapshotFlags |= PlayerSnapshotFlagUpdatedClientMovement
}

func (pk *PlayerSnapshot) SetCRot(rot mgl32.Vec3) {
	pk.CRot = rot
	pk.SnapshotFlags |= PlayerSnapshotFlagUpdatedRotation
}

func (pk *PlayerSnapshot) CheckFlag(flag uint16) bool {
	return pk.SnapshotFlags&flag == flag
}
