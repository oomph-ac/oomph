package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
)

func IsWall(b world.Block) bool {
	_, isWall := b.(block.Wall)
	return isWall
}
