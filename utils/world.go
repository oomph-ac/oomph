package utils

import (
	"math"
	_ "unsafe"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/game"
	oomph_world "github.com/oomph-ac/oomph/world"
	"github.com/oomph-ac/oomph/world/blockmodel"
)

type BlockSearchResult struct {
	Block    world.Block
	Position cube.Pos
}

// BlockName returns the name of the block.
func BlockName(b world.Block) string {
	n, _ := b.EncodeBlock()
	return n
}

// BlockFriction returns the friction of the block.
func BlockFriction(b world.Block) float32 {
	if f, ok := b.(block.Frictional); ok {
		return float32(f.Friction())
	}

	switch BlockName(b) {
	case "minecraft:slime":
		return 0.8
	case "minecraft:ice", "minecraft:packed_ice":
		return 0.98
	case "minecraft:blue_ice":
		return 0.99
	default:
		return 0.6
	}
}

// CanPassBlock returns true if an entity can pass through the given block.
func CanPassBlock(b world.Block) bool {
	switch BlockName(b) {
	case "minecraft:web":
		return true
	default:
		return false
	}
}

// OneWayCollisionBlocks returns an array of blocks that utilize one-way physics.
func OneWayCollisionBlocks(blocks []BlockSearchResult) []world.Block {
	oneWayBlocks := []world.Block{}
	for _, b := range blocks {
		if BlockClimbable(b.Block) {
			oneWayBlocks = append(oneWayBlocks, b.Block)
		}
	}

	return oneWayBlocks
}

// BlockBoxes returns the bounding boxes of the given block based on it's name.
func BlockBoxes(b world.Block, pos cube.Pos, w *oomph_world.World) []cube.BBox {
	switch BlockName(b) {
	case "minecraft:portal", "minecraft:end_portal":
		return []cube.BBox{}
	case "minecraft:web":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	case "minecraft:bed":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 9.0/16.0, 1)}
	case "minecraft:waterlily":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1.0/64.0, 1)}
	case "minecraft:soul_sand":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 7.0/8.0, 1)}
	case "minecraft:snow_layer":
		_, dat := b.EncodeBlock()
		height, ok := dat["height"]
		if !ok {
			return []cube.BBox{}
		}

		blockBBY := float32(height.(int32)) / 8.0
		return []cube.BBox{cube.Box(0, 0, 0, 1, blockBBY, 1)}
	case "minecraft:redstone_ore", "minecraft:redstone_wire":
		return []cube.BBox{}
	case "minecraft:golden_rail", "minecraft:detector_rail", "minecraft:activator_rail", "minecraft:rail":
		return []cube.BBox{}
	case "minecraft:lever":
		return []cube.BBox{}
	case "minecraft:redstone_torch", "minecraft:unlit_redstone_torch":
		return []cube.BBox{}
	case "minecraft:repeater", "minecraft:unpowered_repeater", "minecraft:powered_repeater":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1.0/8.0, 1)}
	case "minecraft:comparator", "minecraft:unpowered_comparator", "minecraft:powered_comparator":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1.0/8.0, 1)}
	case "minecraft:daylight_detector", "minecraft:daylight_detector_inverted":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 3.0/8.0, 1)}
	case "minecraft:bamboo_sapling", "minecraft:bamboo":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	case "minecraft:vine", "minecraft:cave_vines", "minecraft:cave_vines_body_with_berries", "minecraft:cave_vines_head_with_berries",
		"minecraft:twisting_vines", "minecraft:weeping_vines":
		return []cube.BBox{}
	case "minecraft:flower_pot":
		return []cube.BBox{cube.Box(5/16.0, 0, 5/16.0, 11/16.0, 3/8.0, 11/16.0)}
	case "minecraft:black_candle":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	case "minecraft:tallgrass", "minecraft:fern", "minecraft:large_fern", "minecraft:rose_bush", "minecraft:peony", "minecraft:paeonia":
		return []cube.BBox{}
	case "minecraft:end_portal_frame":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 13.0/16.0, 1)}
	case "minecraft:red_mushroom", "minecraft:brown_mushroom":
		return []cube.BBox{}
	}

	// This is used whenever a block is already registered by DF, but the bounding boxes produced by the
	var m world.BlockModel
	switch oldModel := b.Model().(type) {
	case model.Wall:
		m = blockmodel.Wall{
			NorthConnection: oldModel.NorthConnection,
			EastConnection:  oldModel.EastConnection,
			SouthConnection: oldModel.SouthConnection,
			WestConnection:  oldModel.WestConnection,
			Post:            oldModel.Post,
		}
	default:
		switch b.(type) {
		case block.IronBars:
			m = blockmodel.IronBars{}
		default:
			m = oldModel
		}
	}

	dfBoxes := m.BBox(df_cube.Pos(pos), w)
	var boxes = make([]cube.BBox, len(dfBoxes))
	for i, bb := range dfBoxes {
		boxes[i] = game.DFBoxToCubeBox(bb)
	}

	return boxes
}

// GetNearbyBlocks get the blocks that are within a range of the provided bounding box.
func GetNearbyBlocks(aabb cube.BBox, includeAir bool, includeUnknown bool, w *oomph_world.World) []BlockSearchResult {
	grown := aabb.Grow(0.5)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))

	blocks := make([]BlockSearchResult, 0, (maxX-minX)*(maxY-minY)*(maxZ-minZ))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				b := w.Block(df_cube.Pos(pos))
				if _, isAir := b.(block.Air); !includeAir && isAir {
					b = nil
					continue
				}

				// If the hash is MaxUint64, then the block is unknown to dragonfly.
				bHash, _ := b.Hash()
				if !includeUnknown && bHash == math.MaxUint64 {
					b = nil
					continue
				}

				// Add the block to the list of block search results.
				blocks = append(blocks, BlockSearchResult{
					Block:    b,
					Position: pos,
				})
				b = nil
			}
		}
	}

	return blocks
}

// GetNearbyBBoxes returns a list of block bounding boxes that are within the given bounding box.
func GetNearbyBBoxes(aabb cube.BBox, w *oomph_world.World) []cube.BBox {
	grown := aabb.Grow(0.5)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))

	// A prediction of one BBox per block, plus an additional 2, in case
	var bboxList []cube.BBox
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				block := w.Block(df_cube.Pos(pos))

				for _, box := range BlockBoxes(block, pos, w) {
					b := box.Translate(pos.Vec3())
					if !b.IntersectsWith(aabb) || CanPassBlock(block) {
						continue
					}

					bboxList = append(bboxList, b)
				}
			}
		}
	}

	return bboxList
}

// BlockClimbable returns whether the given block is climbable.
func BlockClimbable(b world.Block) bool {
	switch b.(type) {
	case block.Ladder:
		return true
	}

	switch BlockName(b) {
	case "minecraft:vine", "minecraft:cave_vines", "minecraft:cave_vines_body_with_berries", "minecraft:cave_vines_head_with_berries",
		"minecraft:twisting_vines", "minecraft:weeping_vines":
		return true
	default:
		return false
	}
}

// IsBlockPassInteraction returns true if the block allows interactions although it has a solid
// collision bounding box.
func IsBlockPassInteraction(b world.Block) bool {
	switch BlockName(b) {
	case "minecraft:barrier", "minecraft:invisible_bedrock":
		return true
	default:
		return false
	}
}

// BlockToCubePos converts protocol.BlockPos into cube.Pos
func BlockToCubePos(p [3]int32) cube.Pos {
	return cube.Pos{int(p[0]), int(p[1]), int(p[2])}
}
