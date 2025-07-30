package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func init() {
	Register(IDBlockInteractionSnapshot, func() Packet {
		return &BlockInteractionSnapshot{}
	})
}

type BlockInteractionSnapshot struct {
	IsInital bool // 1 byte

	ActionType  protocol.Optional[uint32] // 1-6 bytes
	TriggerType protocol.Optional[uint32] // 1-6 bytes
	CPrediction protocol.Optional[uint32] // 1-6 bytes

	BlockFace protocol.Optional[int32]             // 1-6 bytes
	BlockPos  protocol.Optional[protocol.BlockPos] // 1-16 bytes

	ReportedPos protocol.Optional[mgl32.Vec3] // 1-13 bytes
	ClickedPos  protocol.Optional[mgl32.Vec3] // 1-13 bytes
}

func (*BlockInteractionSnapshot) ID() uint32 {
	return IDBlockInteractionSnapshot
}

func (pk *BlockInteractionSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Bool(&pk.IsInital)
	protocol.OptionalFunc(io, &pk.ActionType, io.Varuint32)
	protocol.OptionalFunc(io, &pk.TriggerType, io.Varuint32)
	protocol.OptionalFunc(io, &pk.CPrediction, io.Varuint32)
	protocol.OptionalFunc(io, &pk.BlockFace, io.Varint32)
	protocol.OptionalFunc(io, &pk.BlockPos, io.BlockPos)
	protocol.OptionalFunc(io, &pk.ReportedPos, io.Vec3)
	protocol.OptionalFunc(io, &pk.ClickedPos, io.Vec3)
}
