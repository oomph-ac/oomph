package utils

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// BlockClimbable returns whether the given block is climbable.
func BlockClimbable(b world.Block) bool {
	switch b.(type) {
	case block.Ladder:
		return true
		// TODO: Add vines here.
	}
	return false
}

func BoxesIntersect(bb cube.BBox, boxes []cube.BBox, bpos mgl64.Vec3) bool {
	for _, box := range boxes {
		if box.Translate(bpos).IntersectsWith(bb) {
			return false
		}
	}

	return true
}
