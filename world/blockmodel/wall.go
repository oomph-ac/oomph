package blockmodel

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type Wall struct {
	// NorthConnection is the height of the connection for the north direction.
	NorthConnection float64
	// EastConnection is the height of the connection for the east direction.
	EastConnection float64
	// SouthConnection is the height of the connection for the south direction.
	SouthConnection float64
	// WestConnection is the height of the connection for the west direction.
	WestConnection float64
	// Post is if the wall is the full height of a block or not.
	Post bool
}

func (w Wall) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	var (
		north, south bool = w.NorthConnection > 0, w.SouthConnection > 0
		west, east   bool = w.WestConnection > 0, w.EastConnection > 0

		inset float64 = 0.25

		box cube.BBox = cube.Box(0, 0, 0, 1, 1.5, 1)
	)

	if !w.Post && ((north && south && !west && !east) || (!north && !south && west && east)) {
		inset = 0.3125
	}

	if !north {
		box = box.ExtendTowards(cube.FaceNorth, -inset)
	}
	if !south {
		box = box.ExtendTowards(cube.FaceSouth, -inset)
	}
	if !west {
		box = box.ExtendTowards(cube.FaceWest, -inset)
	}
	if !east {
		box = box.ExtendTowards(cube.FaceEast, -inset)
	}

	return []cube.BBox{box}
}

// FaceSolid returns true if the face is in the Y axis.
func (w Wall) FaceSolid(_ cube.Pos, face cube.Face, _ world.BlockSource) bool {
	return face.Axis() == cube.Y
}
