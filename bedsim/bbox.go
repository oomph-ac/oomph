package bedsim

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// BoundingBox returns the entity bounding box translated to the current position.
func (s *MovementState) BoundingBox(useSlideOffset bool) cube.BBox {
	scale := s.Size[2]
	width := (s.Size[0] * 0.5) * scale
	height := s.Size[1] * scale
	yOffset := 0.0
	if useSlideOffset {
		yOffset = s.SlideOffset.Y()
	}

	return cube.Box(
		s.Pos[0]-width,
		s.Pos[1]+yOffset,
		s.Pos[2]-width,
		s.Pos[0]+width,
		s.Pos[1]+height+yOffset,
		s.Pos[2]+width,
	).GrowVec3(mgl64.Vec3{-1e-4, 0, -1e-4})
}

// ClientBoundingBox returns the bounding box translated to the client's position.
func (s *MovementState) ClientBoundingBox(useSlideOffset bool) cube.BBox {
	width := s.Size[0] / 2
	yOffset := 0.0
	if useSlideOffset {
		yOffset = s.SlideOffset.Y()
	}

	return cube.Box(
		s.Client.Pos[0]-width,
		s.Client.Pos[1]+yOffset,
		s.Client.Pos[2]-width,
		s.Client.Pos[0]+width,
		s.Client.Pos[1]+s.Size[1]+yOffset,
		s.Client.Pos[2]+width,
	).GrowVec3(mgl64.Vec3{-1e-4, 0, -1e-4})
}
