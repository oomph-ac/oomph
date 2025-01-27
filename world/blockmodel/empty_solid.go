package blockmodel

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type NoCollisionSolid struct{}

func (pp NoCollisionSolid) BBox(pos cube.Pos, s world.BlockSource) []cube.BBox {
	return []cube.BBox{}
}

func (pp NoCollisionSolid) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return true
}
