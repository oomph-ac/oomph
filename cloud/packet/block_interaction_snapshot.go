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
	ActionType  uint32 // 1-5 bytes
	TriggerType uint32 // 1-5 bytes
	CPrediction uint32 // 1-5 bytes

	BlockFace int32             // 1-5 bytes
	BlockPos  protocol.BlockPos // 3-15 bytes

	ReportedPos mgl32.Vec3 // 12 bytes
	ClickedPos  mgl32.Vec3 // 12 bytes
}

func (*BlockInteractionSnapshot) ID() uint32 {
	return IDBlockInteractionSnapshot
}

func (pk *BlockInteractionSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Varuint32(&pk.ActionType)
	io.Varuint32(&pk.TriggerType)
	io.Varuint32(&pk.CPrediction)
	io.Varint32(&pk.BlockFace)
	io.BlockPos(&pk.BlockPos)
	io.Vec3(&pk.ReportedPos)
	io.Vec3(&pk.ClickedPos)
}
