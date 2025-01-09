package blockmodel

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type IronBars struct{}

func (ib IronBars) BBox(pos cube.Pos, s world.BlockSource) (bbs []cube.BBox) {
	inset := float64(7.0 / 16.0)
	connectWest, connectEast := ib.checkConnection(pos, cube.FaceWest, s), ib.checkConnection(pos, cube.FaceEast, s)
	if connectWest || connectEast {
		bb := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.Z, -inset)
		if !connectWest {
			bb = bb.ExtendTowards(cube.FaceWest, -inset)
		} else if !connectEast {
			bb = bb.ExtendTowards(cube.FaceEast, -inset)
		}
		bbs = append(bbs, bb)
	}

	connectNorth, connectSouth := ib.checkConnection(pos, cube.FaceNorth, s), ib.checkConnection(pos, cube.FaceSouth, s)
	if connectNorth || connectSouth {
		bb := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.X, -inset)
		if !connectNorth {
			bb = bb.ExtendTowards(cube.FaceNorth, -inset)
		} else if !connectSouth {
			bb = bb.ExtendTowards(cube.FaceSouth, -inset)
		}
		bbs = append(bbs, bb)
	}

	// This will happen if there are no connections in any direction.
	if len(bbs) == 0 {
		bbs = append(bbs, cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.X, -inset).Stretch(cube.Z, -inset))
	}
	return
}

func (ib IronBars) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return true
}

func (ib IronBars) checkConnection(pos cube.Pos, f cube.Face, s world.BlockSource) bool {
	b := s.Block(pos.Side(f))
	if _, isIronBar := b.(block.IronBars); isIronBar {
		return true
	} else if _, isLeaves := b.(block.Leaves); isLeaves {
		return false
	}
	// TODO: check for walls, as they are able to connect with iron bars.

	boxCount := 0
	for _, bb := range b.Model().BBox(pos.Side(f), s) {
		boxCount++
		if bb.Width() != 1 || bb.Height() != 1 || bb.Length() != 1 {
			return false
		}
	}
	return boxCount > 0
}
