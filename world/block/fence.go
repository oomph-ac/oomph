package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
)

func IsFence(b world.Block) bool {
	switch b.(type) {
	case block.WoodFence, block.WoodFenceGate, block.NetherBrickFence:
		return true
	default:
		return false
	}
}
