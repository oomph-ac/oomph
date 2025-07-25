package blockmodel

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type IronBars struct{}

func (ib IronBars) BBox(pos cube.Pos, s world.BlockSource) (bbs []cube.BBox) {
	const insetDefault = 7.0 / 16.0
	const insetConnecting = 8.0 / 16.0

	connectWest, connectEast := ib.checkConnection(pos, cube.FaceWest, s), ib.checkConnection(pos, cube.FaceEast, s)
	if connectWest || connectEast {
		bb := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.Z, -insetDefault)
		if !connectWest {
			bb = bb.ExtendTowards(cube.FaceWest, -insetConnecting)
		} else if !connectEast {
			bb = bb.ExtendTowards(cube.FaceEast, -insetConnecting)
		}
		bbs = append(bbs, bb)
	}

	connectNorth, connectSouth := ib.checkConnection(pos, cube.FaceNorth, s), ib.checkConnection(pos, cube.FaceSouth, s)
	if connectNorth || connectSouth {
		bb := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.X, -insetDefault)
		if !connectNorth {
			bb = bb.ExtendTowards(cube.FaceNorth, -insetConnecting)
		} else if !connectSouth {
			bb = bb.ExtendTowards(cube.FaceSouth, -insetConnecting)
		}
		bbs = append(bbs, bb)
	}

	// This will happen if there are no connections in any direction.
	if len(bbs) == 0 {
		bbs = append(bbs, cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.X, -insetDefault).Stretch(cube.Z, -insetDefault))
	}
	return
}

func (ib IronBars) FaceSolid(pos cube.Pos, face cube.Face, s world.BlockSource) bool {
	return true
}

func (ib IronBars) checkConnection(pos cube.Pos, f cube.Face, s world.BlockSource) bool {
	sidePos := pos.Side(f)
	b := s.Block(sidePos)
	if _, isIronBar := b.(block.IronBars); isIronBar {
		return true
	} else if _, isWall := b.(block.Wall); isWall {
		return true
	}
	return b.Model().FaceSolid(sidePos, f.Opposite(), s)

}
