package blockmodel

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

var (
	oneBB = cube.Box(
		7.0/16.0,
		0.0,
		7.0/16.0,
		9.0/16.0,
		6.0/16.0,
		9.0/16.0,
	)
	twoBB = cube.Box(
		0.3125,
		0.0,
		0.4375,
		0.6875,
		6.0/16.0,
		0.625,
	)
	threeBB = cube.Box(
		0.3125,
		0.0,
		0.375,
		0.625,
		6.0/16.0,
		0.6875,
	)
	fourBB = cube.Box(
		0.3125,
		0.0,
		0.3125,
		0.6875,
		6.0/16.0,
		0.625,
	)
)

type Candle struct {
	Count int32
	Lit   bool
}

func (c Candle) BBox(pos cube.Pos, s world.BlockSource) []cube.BBox {
	var bb cube.BBox
	switch c.Count {
	case 1:
		bb = oneBB
	case 2:
		bb = twoBB
	case 3:
		bb = threeBB
	case 4:
		bb = fourBB
	default:
		panic(fmt.Errorf("invalid count for candles (%d)", c.Count))
	}
	return []cube.BBox{bb}
}

func (c Candle) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return false
}
