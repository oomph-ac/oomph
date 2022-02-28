package utils

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"math"
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

// BlocksNearby returns a slice of blocks that are nearby the search position.
func BlocksNearby(pos mgl64.Vec3, aabb physics.AABB, w *world.World) []world.Block {
	grown := aabb.Grow(1)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math.Floor(min[0])), int(math.Floor(min[1])), int(math.Floor(min[2]))
	maxX, maxY, maxZ := int(math.Ceil(max[0])), int(math.Ceil(max[1])), int(math.Ceil(max[2]))
	blocks := make([]world.Block, 0, (maxX-minX)*(maxY-minY)*(maxZ-minZ))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				blocks = append(blocks, w.Block(cube.PosFromVec3(pos).Add(cube.Pos{x, y, z})))
			}
		}
	}
	return blocks
}

// CollidingBlocks returns all block AABBs that collide with the entity.
func CollidingBlocks(aabb physics.AABB, pos mgl64.Vec3, w *world.World) []physics.AABB {
	grown := aabb.Grow(1)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math.Floor(min[0])), int(math.Floor(min[1])), int(math.Floor(min[2]))
	maxX, maxY, maxZ := int(math.Ceil(max[0])), int(math.Ceil(max[1])), int(math.Ceil(max[2]))

	blockAABBs := make([]physics.AABB, 0, (maxX-minX)*(maxY-minY)*(maxZ-minZ))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				blockPos := cube.Pos{x, y, z}
				if b := w.Block(blockPos); !PassThroughBlock(aabb, pos, blockPos, w) {
					for _, bb := range b.Model().AABB(blockPos, w) {
						if bb.Translate(blockPos.Vec3()).IntersectsWith(aabb) {
							blockAABBs = append(blockAABBs, bb)
						}
					}
				}
			}
		}
	}
	return blockAABBs
}

// PassThroughBlock returns true if the entity can pass through the block at the given position.
func PassThroughBlock(aabb physics.AABB, pos mgl64.Vec3, blockPos cube.Pos, w *world.World) bool {
	for _, bb := range w.Block(blockPos).Model().AABB(blockPos, w) {
		if bb.Grow(0.05).Translate(blockPos.Vec3()).IntersectsWith(aabb.Translate(pos)) {
			return true
		}
	}
	return false
}
