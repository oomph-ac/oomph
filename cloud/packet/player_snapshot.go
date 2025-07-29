package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	gtpacket "github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	SimFlagInCorrectiveState = iota
	SimFlagHasTeleport
	SimFlagHasKnockback
	SimFlagsSize
)

func init() {
	Register(IDPlayerSnapshot, func() Packet {
		return &PlayerSnapshot{}
	})
}

type PlayerSnapshot struct {
	Pos, CPos mgl32.Vec3 // 24 bytes
	Vel, CVel mgl32.Vec3 // 24 bytes
	Mov, CMov mgl32.Vec3 // 24 bytes
	CRot      mgl32.Vec3 // 12 bytes

	CInputFlags protocol.Bitset // 9 bytes (?)
	SimFlags    protocol.Bitset // ???

	Timestamp  int64  // 8 bytes
	CInputTick int64  // 8 bytes
	CSimTick   uint64 // 8 bytes

}

func (*PlayerSnapshot) ID() uint32 {
	return IDPlayerSnapshot
}

func (pk *PlayerSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Vec3(&pk.Pos)
	io.Vec3(&pk.Vel)
	io.Vec3(&pk.Mov)
	io.Vec3(&pk.CPos)
	io.Vec3(&pk.CVel)
	io.Vec3(&pk.CMov)
	io.Vec3(&pk.CRot)
	io.Bitset(&pk.CInputFlags, gtpacket.PlayerAuthInputBitsetSize)
	io.Bitset(&pk.SimFlags, SimFlagsSize)
	io.Int64(&pk.Timestamp)
	io.Int64(&pk.CInputTick)
	io.Uint64(&pk.CSimTick)
}
