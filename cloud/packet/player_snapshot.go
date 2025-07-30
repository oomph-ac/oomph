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
	Pos, CPos protocol.Optional[mgl32.Vec3] // 2-25 bytes
	Vel, CVel protocol.Optional[mgl32.Vec3] // 2-25 bytes
	Mov, CMov protocol.Optional[mgl32.Vec3] // 2-25 bytes
	CRot      protocol.Optional[mgl32.Vec3] // 1-13 bytes

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
	protocol.OptionalFunc(io, &pk.Pos, io.Vec3)
	protocol.OptionalFunc(io, &pk.Vel, io.Vec3)
	protocol.OptionalFunc(io, &pk.Mov, io.Vec3)
	protocol.OptionalFunc(io, &pk.CPos, io.Vec3)
	protocol.OptionalFunc(io, &pk.CVel, io.Vec3)
	protocol.OptionalFunc(io, &pk.CMov, io.Vec3)
	protocol.OptionalFunc(io, &pk.CRot, io.Vec3)
	io.Bitset(&pk.CInputFlags, gtpacket.PlayerAuthInputBitsetSize)
	io.Bitset(&pk.SimFlags, SimFlagsSize)
	io.Int64(&pk.Timestamp)
	io.Int64(&pk.CInputTick)
	io.Uint64(&pk.CSimTick)
}
