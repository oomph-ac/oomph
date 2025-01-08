package blockmodel

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type PressurePlate struct{}

func (pp PressurePlate) BBox(pos cube.Pos, s world.BlockSource) []cube.BBox {
	return []cube.BBox{}
}

func (pp PressurePlate) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return true
}
