package model

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/go-gl/mathgl/mgl64"
)

// Stair is a model for stair-like blocks. These have different solid sides depending on the direction the
// stairs are facing, the surrounding blocks and whether it is upside down or not.
type Stair struct {
	model.Stair
}

// AABB returns a slice of physics.AABB depending on if the Stair is upside down and which direction it is facing.
// Additionally, these AABBs depend on the Stair blocks surrounding this one, which can influence the model.
func (s Stair) AABB(pos cube.Pos, w HasWorld) []physics.AABB {
	b := []physics.AABB{physics.NewAABB(mgl64.Vec3{}, mgl64.Vec3{1, 0.5, 1})}
	if s.UpsideDown {
		b[0] = physics.NewAABB(mgl64.Vec3{0, 0.5, 0}, mgl64.Vec3{1, 1, 1})
	}
	t := s.cornerType(pos, w)

	face, oppositeFace := s.Facing.Face(), s.Facing.Opposite().Face()
	if t == noCorner || t == cornerRightInner || t == cornerLeftInner {
		b = append(b, physics.NewAABB(mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0.5, 1, 0.5}).
			ExtendTowards(face, 0.5).
			Stretch(s.Facing.RotateRight().Face().Axis(), 0.5))
	}
	if t == cornerRightOuter {
		b = append(b, physics.NewAABB(mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0.5, 1, 0.5}).
			ExtendTowards(face, 0.5).
			ExtendTowards(s.Facing.RotateLeft().Face(), 0.5))
	} else if t == cornerLeftOuter {
		b = append(b, physics.NewAABB(mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0.5, 1, 0.5}).
			ExtendTowards(face, 0.5).
			ExtendTowards(s.Facing.RotateRight().Face(), 0.5))
	} else if t == cornerRightInner {
		b = append(b, physics.NewAABB(mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0.5, 1, 0.5}).
			ExtendTowards(oppositeFace, 0.5).
			ExtendTowards(s.Facing.RotateRight().Face(), 0.5))
	} else if t == cornerLeftInner {
		b = append(b, physics.NewAABB(mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0.5, 1, 0.5}).
			ExtendTowards(oppositeFace, 0.5).
			ExtendTowards(s.Facing.RotateLeft().Face(), 0.5))
	}
	if s.UpsideDown {
		for i := range b[1:] {
			b[i+1] = b[i+1].Translate(mgl64.Vec3{0, -0.5})
		}
	}
	return b
}

const (
	noCorner = iota
	cornerRightInner
	cornerLeftInner
	cornerRightOuter
	cornerLeftOuter
)

// cornerType returns the type of the corner that the stairs form, or 0 if it does not form a corner with any
// other stairs.
func (s Stair) cornerType(pos cube.Pos, w HasWorld) uint8 {
	rotatedFacing := s.Facing.RotateRight()
	if closedSide, ok := w.Block(pos.Side(s.Facing.Face())).Model().(model.Stair); ok && closedSide.UpsideDown == s.UpsideDown {
		if closedSide.Facing == rotatedFacing {
			return cornerLeftOuter
		} else if closedSide.Facing == rotatedFacing.Opposite() {
			// This will only form a corner if there is not a stair on the right of this one with the same
			// direction.
			if side, ok := w.Block(pos.Side(s.Facing.RotateRight().Face())).Model().(model.Stair); !ok || side.Facing != s.Facing || side.UpsideDown != s.UpsideDown {
				return cornerRightOuter
			}
			return noCorner
		}
	}
	if openSide, ok := w.Block(pos.Side(s.Facing.Opposite().Face())).Model().(model.Stair); ok && openSide.UpsideDown == s.UpsideDown {
		if openSide.Facing == rotatedFacing {
			// This will only form a corner if there is not a stair on the right of this one with the same
			// direction.
			if side, ok := w.Block(pos.Side(s.Facing.RotateRight().Face())).Model().(model.Stair); !ok || side.Facing != s.Facing || side.UpsideDown != s.UpsideDown {
				return cornerRightInner
			}
		} else if openSide.Facing == rotatedFacing.Opposite() {
			return cornerLeftInner
		}
	}
	return noCorner
}
