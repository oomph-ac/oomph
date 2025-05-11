package blockmodel

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type Candle struct {
	Count int32
	Lit   bool
}

func (c Candle) BBox(pos cube.Pos, s world.BlockSource) []cube.BBox {
	const inset1 = 7.0 / 16.0
	const inset2 = 6.0 / 16.0
	const inset3 = 5.0 / 16.0
	const downardInset = 10.0 / 16.0

	bb := cube.Box(0, 0, 0, 1, 1, 1)
	switch c.Count {
	case 1:
		bb = bb.Stretch(cube.X, -inset1).
			Stretch(cube.Z, -inset1)
	case 2:
		bb = bb.Stretch(cube.X, -inset3).
			ExtendTowards(cube.FaceUp, -inset1).
			ExtendTowards(cube.FaceDown, -inset2)
	case 3:
		bb = bb.ExtendTowards(cube.FaceWest, -inset3).
			ExtendTowards(cube.FaceEast, -inset2).
			ExtendTowards(cube.FaceNorth, -inset2).
			ExtendTowards(cube.FaceSouth, -inset3)
	case 4:
		bb = bb.Stretch(cube.X, -inset3).
			ExtendTowards(cube.FaceNorth, -inset3).
			ExtendTowards(cube.FaceSouth, -inset2)
	default:
		panic(fmt.Errorf("invalid count for candles (%d)", c.Count))
	}
	return []cube.BBox{bb.ExtendTowards(cube.FaceUp, -downardInset)}
}

func (c Candle) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return true
}
