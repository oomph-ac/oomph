package world

import (
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
)

// SetBlockTransaction is a transaction that sets a block at a position.
type SetBlockTransaction struct {
	BlockPos       cube.Pos
	BlockRuntimeId uint32
}

func (tx *SetBlockTransaction) Execute(c *chunk.Chunk) {
	c.SetBlock(uint8(tx.BlockPos[0]), int16(tx.BlockPos[1]), uint8(tx.BlockPos[2]), 0, tx.BlockRuntimeId)
}
