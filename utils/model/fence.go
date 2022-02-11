package model

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/go-gl/mathgl/mgl64"
)

// Fence is a model used by fences of any type. It can attach to blocks with solid faces and other fences of the same
// type and has a model height just slightly over 1.
type Fence struct {
	model.Fence
}

// AABB returns multiple physics.AABB depending on how many connections it has with the surrounding blocks.
func (f Fence) AABB(pos cube.Pos, w HasWorld) []physics.AABB {
	const offset = 0.375

	boxes := make([]physics.AABB, 0, 5)
	mainBox := physics.NewAABB(mgl64.Vec3{offset, 0, offset}, mgl64.Vec3{1 - offset, 1.5, 1 - offset})

	for i := cube.Face(2); i < 6; i++ {
		pos := pos.Side(i)
		block := w.Block(pos)

		if fence, ok := block.Model().(model.Fence); (ok && fence.Wood == f.Wood) || block.Model().FaceSolid(pos, i, nil) {
			boxes = append(boxes, mainBox.ExtendTowards(i, offset))
		} else if _, ok := block.Model().(model.FenceGate); ok {
			boxes = append(boxes, mainBox.ExtendTowards(i, offset))
		}
	}
	return append(boxes, mainBox)
}

// FaceSolid returns true if the face is cube.FaceDown or cube.FaceUp.
func (f Fence) FaceSolid(_ cube.Pos, face cube.Face, _ HasWorld) bool {
	return face == cube.FaceDown || face == cube.FaceUp
}
