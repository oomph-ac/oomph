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

const (
	BlockInteractionSnapshotFlagIsInital = 1 << iota
	BlockInteractionSnapshotFlagUpdatedActionType
	BlockInteractionSnapshotFlagUpdatedTriggerType
	BlockInteractionSnapshotFlagUpdatedClientPrediction
	BlockInteractionSnapshotFlagUpdatedBlockFace
	BlockInteractionSnapshotFlagUpdatedBlockPos
	BlockInteractionSnapshotFlagUpdatedReportedPos
	BlockInteractionSnapshotFlagUpdatedClickedPos
)

type BlockInteractionSnapshot struct {
	Flags uint8 // 1 byte

	ActionType  uint32 // 0-5 bytes
	TriggerType uint32 // 0-5 bytes
	CPrediction uint32 // 0-5 bytes

	BlockFace int32             // 0-5 bytes
	BlockPos  protocol.BlockPos // 0-16 bytes

	ReportedPos mgl32.Vec3 // 0-12 bytes
	ClickedPos  mgl32.Vec3 // 0-12 bytes
}

func (*BlockInteractionSnapshot) ID() uint32 {
	return IDBlockInteractionSnapshot
}

func (pk *BlockInteractionSnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.Flags)
	if pk.CheckFlag(BlockInteractionSnapshotFlagIsInital) {
		io.Varuint32(&pk.ActionType)
		io.Varuint32(&pk.TriggerType)
		io.Varuint32(&pk.CPrediction)
		io.Varint32(&pk.BlockFace)
		io.BlockPos(&pk.BlockPos)
		io.Vec3(&pk.ReportedPos)
		io.Vec3(&pk.ClickedPos)
		return
	}

	if pk.CheckFlag(BlockInteractionSnapshotFlagUpdatedActionType) {
		io.Varuint32(&pk.ActionType)
	}
	if pk.CheckFlag(BlockInteractionSnapshotFlagUpdatedTriggerType) {
		io.Varuint32(&pk.TriggerType)
	}
	if pk.CheckFlag(BlockInteractionSnapshotFlagUpdatedClientPrediction) {
		io.Varuint32(&pk.CPrediction)
	}
	if pk.CheckFlag(BlockInteractionSnapshotFlagUpdatedBlockFace) {
		io.Varint32(&pk.BlockFace)
	}
	if pk.CheckFlag(BlockInteractionSnapshotFlagUpdatedBlockPos) {
		io.BlockPos(&pk.BlockPos)
	}
	if pk.CheckFlag(BlockInteractionSnapshotFlagUpdatedReportedPos) {
		io.Vec3(&pk.ReportedPos)
	}
	if pk.CheckFlag(BlockInteractionSnapshotFlagUpdatedClickedPos) {
		io.Vec3(&pk.ClickedPos)
	}
}

func (pk *BlockInteractionSnapshot) SetActionType(actionType uint32) {
	pk.ActionType = actionType
	pk.Flags |= BlockInteractionSnapshotFlagUpdatedActionType
}

func (pk *BlockInteractionSnapshot) SetTriggerType(triggerType uint32) {
	pk.TriggerType = triggerType
	pk.Flags |= BlockInteractionSnapshotFlagUpdatedTriggerType
}

func (pk *BlockInteractionSnapshot) SetClientPrediction(clientPrediction uint32) {
	pk.CPrediction = clientPrediction
	pk.Flags |= BlockInteractionSnapshotFlagUpdatedClientPrediction
}

func (pk *BlockInteractionSnapshot) SetBlockFace(blockFace int32) {
	pk.BlockFace = blockFace
	pk.Flags |= BlockInteractionSnapshotFlagUpdatedBlockFace
}

func (pk *BlockInteractionSnapshot) SetBlockPos(blockPos protocol.BlockPos) {
	pk.BlockPos = blockPos
	pk.Flags |= BlockInteractionSnapshotFlagUpdatedBlockPos
}

func (pk *BlockInteractionSnapshot) SetReportedPos(reportedPos mgl32.Vec3) {
	pk.ReportedPos = reportedPos
	pk.Flags |= BlockInteractionSnapshotFlagUpdatedReportedPos
}

func (pk *BlockInteractionSnapshot) SetClickedPos(clickedPos mgl32.Vec3) {
	pk.ClickedPos = clickedPos
	pk.Flags |= BlockInteractionSnapshotFlagUpdatedClickedPos
}

func (pk *BlockInteractionSnapshot) CheckFlag(flag uint8) bool {
	return pk.Flags&flag == flag
}
