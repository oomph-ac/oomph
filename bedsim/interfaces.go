package bedsim

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// WorldProvider bridges the world/chunk system for collision and block lookups.
type WorldProvider interface {
	Block(pos cube.Pos) world.Block
	BlockCollisions(pos cube.Pos) []cube.BBox
	GetNearbyBBoxes(aabb cube.BBox) []cube.BBox
	IsChunkLoaded(chunkX, chunkZ int32) bool
}

// EffectsProvider bridges effect tracking (jump boost, levitation, slow falling, etc.).
type EffectsProvider interface {
	GetEffect(effectID int32) (amplifier int32, ok bool)
}

// InventoryProvider exposes equipment checks needed by movement (elytra, etc.).
type InventoryProvider interface {
	HasElytra() bool
}
