package model

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type HasWorld interface {
	// Block reads a block from the position passed. If a chunk is not yet loaded at that position, it will return air.
	Block(pos cube.Pos) world.Block
}
