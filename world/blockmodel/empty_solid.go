package blockmodel

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

var emptyBBList = []cube.BBox{}

type NoCollisionSolid struct{}

func (pp NoCollisionSolid) BBox(pos cube.Pos, s world.BlockSource) []cube.BBox {
	return emptyBBList
}

func (pp NoCollisionSolid) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return true
}

type NoCollisionNotSolid struct{}

func (pp NoCollisionNotSolid) BBox(pos cube.Pos, s world.BlockSource) []cube.BBox {
	return emptyBBList
}

func (pp NoCollisionNotSolid) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return false
}
