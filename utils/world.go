package utils

import (
	"iter"
	"math"
	_ "unsafe"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/world/blockmodel"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type BlockSearchResult struct {
	Block    world.Block
	Position cube.Pos
}

var (
	blockNameMapping = map[uint64]string{}
)

func InitializeBlockNameMapping() {
	blockNameMapping = make(map[uint64]string, len(world.Blocks()))
	for _, b := range world.Blocks() {
		if x, y := b.Hash(); x == 0 && y == math.MaxUint64 {
			continue
		}
		name, _ := b.EncodeBlock()
		blockNameMapping[world.BlockHash(b)] = name
	}
}

// BlockName returns the name of the block.
func BlockName(b world.Block) string {
	if n, ok := blockNameMapping[world.BlockHash(b)]; ok {
		return n
	}
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
	case "minecraft:web", "minecraft:water", "minecraft:lava":
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
func BlockBoxes(b world.Block, pos cube.Pos, src world.BlockSource) []cube.BBox {
	const (
		sixteenth float32 = 1.0 / 16.0
		eighth    float32 = 1.0 / 8.0
	)

	switch BlockName(b) {
	case "minecraft:portal", "minecraft:end_portal":
		return []cube.BBox{}
	case "minecraft:web":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	case "minecraft:bed":
		return []cube.BBox{cube.Box(sixteenth, 0, sixteenth, 15*sixteenth, 9*sixteenth, 15*sixteenth)}
	case "minecraft:waterlily":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 0.09375, 1)}
	case "minecraft:soul_sand":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 7*eighth, 1)}
	case "minecraft:snow_layer":
		_, dat := b.EncodeBlock()
		height, ok := dat["height"]
		if !ok {
			return []cube.BBox{}
		}

		blockBBY := float32(height.(int32)) * eighth
		return []cube.BBox{cube.Box(0, 0, 0, 1, blockBBY, 1)}
	case "minecraft:redstone_wire":
		return []cube.BBox{}
	case "minecraft:golden_rail", "minecraft:detector_rail", "minecraft:activator_rail", "minecraft:rail":
		return []cube.BBox{}
	case "minecraft:lever":
		return []cube.BBox{}
	case "minecraft:redstone_torch", "minecraft:unlit_redstone_torch":
		return []cube.BBox{}
	case "minecraft:repeater", "minecraft:unpowered_repeater", "minecraft:powered_repeater":
		return []cube.BBox{cube.Box(0, 0, 0, 1, eighth, 1)}
	case "minecraft:comparator", "minecraft:unpowered_comparator", "minecraft:powered_comparator":
		return []cube.BBox{cube.Box(0, 0, 0, 1, eighth, 1)}
	case "minecraft:daylight_detector", "minecraft:daylight_detector_inverted":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 3*eighth, 1)}
	case "minecraft:bamboo_sapling", "minecraft:bamboo":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	case "minecraft:vine", "minecraft:cave_vines", "minecraft:cave_vines_body_with_berries", "minecraft:cave_vines_head_with_berries",
		"minecraft:twisting_vines", "minecraft:weeping_vines":
		return []cube.BBox{}
	case "minecraft:flower_pot":
		return []cube.BBox{cube.Box(5*sixteenth, 0, 5*sixteenth, 11*sixteenth, 3*eighth, 11*sixteenth)}
	case "minecraft:tallgrass", "minecraft:fern", "minecraft:large_fern", "minecraft:rose_bush", "minecraft:peony", "minecraft:paeonia":
		return []cube.BBox{}
	case "minecraft:end_portal_frame":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 13*sixteenth, 1)}
	case "minecraft:red_mushroom", "minecraft:brown_mushroom":
		return []cube.BBox{}
	case "minecraft:glow_lichen", "minecraft:pink_petals":
		return []cube.BBox{}
	}

	var bModel = b.Model()
	if _, isIronBar := b.(block.IronBars); isIronBar {
		bModel = blockmodel.IronBars{}
	}

	var boxes []cube.BBox
	dfBoxes := bModel.BBox(df_cube.Pos(pos), src)
	boxes = make([]cube.BBox, len(dfBoxes))
	for i, bb := range dfBoxes {
		boxes[i] = game.DFBoxToCubeBox(bb)
	}
	return boxes
}

// GetBlocksInRadius returns a list of block positions within a radius of the given position.
func GetBlocksInRadius(pos protocol.BlockPos, radius int32) []protocol.BlockPos {
	blocks := []protocol.BlockPos{}
	for x := -radius; x <= radius; x++ {
		for y := -radius; y <= radius; y++ {
			for z := -radius; z <= radius; z++ {
				blocks = append(blocks, protocol.BlockPos{pos[0] + x, pos[1] + y, pos[2] + z})
			}
		}
	}
	return blocks
}

// GetNearbyBlockCollisions ...
func GetNearbyBlockCollisions(aabb cube.BBox, src world.BlockSource) iter.Seq[BlockSearchResult] {
	return func(yield func(BlockSearchResult) bool) {
		min, max := aabb.Min(), aabb.Max()
		minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
		maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				for z := minZ; z <= maxZ; z++ {
					pos := cube.Pos{x, y, z}
					b := src.Block(df_cube.Pos(pos))
					if _, isAir := b.(block.Air); isAir {
						continue
					}

					// Add the block to the list of block search results.
					vecPos := pos.Vec3()
				block_loop:
					for _, bb := range BlockBoxes(b, pos, src) {
						if bb.Translate(vecPos).IntersectsWith(aabb) {
							if !yield(BlockSearchResult{Block: b, Position: pos}) {
								return
							}
							break block_loop
						}
					}
				}
			}
		}
	}
}

// GetNearbyBlocks get the blocks that are within a range of the provided bounding box.
func GetNearbyBlocks(aabb cube.BBox, includeAir bool, includeUnknown bool, src world.BlockSource) []BlockSearchResult {
	min, max := aabb.Min(), aabb.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))
	blocks := make([]BlockSearchResult, 0, (maxX-minX)*(maxY-minY)*(maxZ-minZ))

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				b := src.Block(df_cube.Pos(pos))
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
			}
		}
	}

	return blocks
}

// GetNearbyBBoxes returns a list of block bounding boxes that are within the given bounding box.
func GetNearbyBBoxes(aabb cube.BBox, src world.BlockSource) []cube.BBox {
	grown := aabb.Grow(1.0)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))
	bboxList := make([]cube.BBox, 0, (maxX-minX)*(maxY-minY)*(maxZ-minZ))

	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			for y := minY; y <= maxY; y++ {
				pos := cube.Pos{x, y, z}
				block := src.Block(df_cube.Pos(pos))
				if CanPassBlock(block) {
					continue
				}

				for _, box := range BlockBoxes(block, pos, src) {
					b := box.Translate(pos.Vec3())
					if b.IntersectsWith(aabb) {
						bboxList = append(bboxList, b)
					}
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

// IsFence returns true if the block is a fence.
func IsFence(b world.Block) bool {
	switch b.(type) {
	case block.WoodFence, block.NetherBrickFence:
		return true
	default:
		return false
	}
}

// IsFenceGate returns true if the block is a fence gate
func IsFenceGate(b world.Block) bool {
	_, ok := b.(block.WoodFenceGate)
	return ok
}

// IsWall returns true if the block is a wall.
func IsWall(b world.Block) bool {
	_, ok := b.(block.Wall)
	return ok
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
