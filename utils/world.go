package utils

import (
	"fmt"
	"math"
	"strings"
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

// IsWall returns true if the given block is a wall.
func IsWall(n string) bool {
	switch n {
	case "minecraft:cobblestone_wall", "minecraft:blackstone_wall", "minecraft:polished_blackstone_wall",
		"minecraft:cobbled_deepslate_wall", "minecraft:polished_blackstone_brick_wall", "minecraft:deepslate_tile_wall",
		"minecraft:mud_brick_wall", "minecraft:polished_deepslate_wall", "minecraft:deepslate_brick_wall":
		return true
	default:
		return false
	}
}

// IsFence returns true if the given block is a fence.
func IsFence(b world.Block) bool {
	if _, ok := b.Model().(model.Fence); ok {
		return true
	}

	if _, ok := b.Model().(model.FenceGate); ok {
		return true
	}

	return false
}

// BlockBoxes returns the bounding boxes of the given block based on it's name.
func BlockBoxes(b world.Block, pos cube.Pos, w *oomph_world.World) []cube.BBox {
	sblocks := map[cube.Face]world.Block{}
	for _, f := range cube.Faces() {
		b := w.Block(df_cube.Pos(pos).Side(df_cube.Face(f)))
		if _, isAir := b.(block.Air); isAir {
			continue
		}

		sblocks[f] = b
	}

	switch BlockName(b) {
	case "minecraft:acacia_button", "minecraft:bamboo_button", "minecraft:birch_button", "minecraft:cherry_button",
		"minecraft:crimson_button", "minecraft:dark_oak_button", "minecraft:jungle_button", "minecraft:mangrove_button",
		"minecraft:polished_blackstone_button", "minecraft:spruce_button", "minecraft:stone_button", "minecraft:warped_button",
		"minecraft:wooden_button":
		return []cube.BBox{}
	case "minecraft:trip_wire":
		return []cube.BBox{}
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
	case "minecraft:iron_bars":
		var bbs []cube.BBox
		connectWest, connectEast, connectNorth, connectSouth := CheckBarConnections(b, w, sblocks)
		inset := float32(7.0 / 16.0)

		if connectWest || connectEast {
			bb := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.Z, -inset)
			if !connectWest {
				bb = bb.ExtendTowards(cube.FaceWest, -inset)
			} else if !connectEast {
				bb = bb.ExtendTowards(cube.FaceEast, -inset)
			}

			bbs = append(bbs, bb)
		}

		if connectNorth || connectSouth {
			bb := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.X, -inset)

			if !connectNorth {
				bb = bb.ExtendTowards(cube.FaceNorth, -inset)
			} else if !connectSouth {
				bb = bb.ExtendTowards(cube.FaceSouth, -inset)
			}

			bbs = append(bbs, bb)
		}

		if len(bbs) == 0 {
			return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1).
				Stretch(cube.X, -inset).
				Stretch(cube.Z, -inset),
			}
		}

		return bbs
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

	if strings.Contains(BlockName(b), "pressure_plate") {
		n, prop := b.EncodeBlock()
		fmt.Printf("%T %s %v\n", b, n, prop)
	}

	var m world.BlockModel
	switch oldM := b.Model().(type) {
	case model.Wall:
		m = blockmodel.Wall{
			NorthConnection: oldM.NorthConnection,
			EastConnection:  oldM.EastConnection,
			SouthConnection: oldM.SouthConnection,
			WestConnection:  oldM.WestConnection,
			Post:            oldM.Post,
		}
	default:
		m = oldM
	}

	dfBoxes := m.BBox(df_cube.Pos(pos), w)
	var boxes = make([]cube.BBox, len(dfBoxes))
	for i, bb := range dfBoxes {
		boxes[i] = game.DFBoxToCubeBox(bb)
	}

	return boxes
}

// IsStair returns true if the block given is a stair block.
func IsStair(n string) bool {
	switch n {
	case "minecraft:prismarine_stairs", "minecraft:dark_prismarine_stairs", "minecraft:prismarine_bricks_stairs", "minecraft:granite_stairs",
		"minecraft:diorite_stairs", "minecraft:andesite_stairs", "minecraft:polished_granite_stairs", "minecraft:polished_diorite_stairs", "minecraft:polished_andesite_stairs",
		"minecraft:mossy_stone_brick_stairs", "minecraft:smooth_red_sandstone_stairs", "minecraft:smooth_sandstone_stairs", "minecraft:end_brick_stairs",
		"minecraft:mossy_cobblestone_stairs", "minecraft:normal_stone_stairs", "minecraft:red_nether_brick_stairs", "minecraft:smooth_quartz_stairs",
		"minecraft:oak_stairs", "minecraft:stone_stairs", "minecraft:brick_stairs", "minecraft:stone_brick_stairs", "minecraft:nether_brick_stairs",
		"minecraft:sandstone_stairs", "minecraft:spruce_stairs", "minecraft:birch_stairs", "minecraft:jungle_stairs", "minecraft:quartz_stairs",
		"minecraft:acacia_stairs", "minecraft:dark_oak_stairs", "minecraft:red_sandstone_stairs", "minecraft:purpur_stairs", "minecraft:mangrove_stairs",
		"minecraft:deepslate_brick_stairs":
		return true
	default:
		return false
	}
}

// Check fence connection checks for connections on the x and z axis the fence may have.
func CheckBarConnections(b world.Block, w *oomph_world.World, sblocks map[cube.Face]world.Block) (bool, bool, bool, bool) {
	// Connections on the X-axis.
	wb, connectWest := sblocks[cube.FaceWest]
	eb, connectEast := sblocks[cube.FaceEast]

	// Check if the block can connect with the fence.
	if connectWest && !BarConnectionCompatiable(wb, w) {
		connectWest = false
	}
	if connectEast && !BarConnectionCompatiable(eb, w) {
		connectEast = false
	}

	// Connections on the Z-axis.
	nb, connectNorth := sblocks[cube.FaceNorth]
	sb, connectSouth := sblocks[cube.FaceSouth]

	// Check if the block can connect with the fence.
	if connectNorth && !BarConnectionCompatiable(nb, w) {
		connectNorth = false
	}
	if connectSouth && !BarConnectionCompatiable(sb, w) {
		connectSouth = false
	}

	return connectWest, connectEast, connectNorth, connectSouth
}

// BarConnectionCompatiable returns true if the given block is compatiable to conenct to a fence.
func BarConnectionCompatiable(b world.Block, w *oomph_world.World) bool {
	n := BlockName(b)
	if n == "minecraft:iron_bars" || IsWall(n) {
		return true
	}

	switch n {
	case "minecraft:azalea_leaves", "minecraft:azalea_leaves_flowered", "minecraft:cherry_leaves", "minecraft:leaves",
		"minecraft:leaves2", "minecraft:mangrove_leaves":
		return false
	default:
		// Assume for now only blocks that are 1x1x1 are compatiable for connections. True for majority of blocks.
		// FIXME: Some non-full cube blocks can still make a connection w/ walls.
		bbs := b.Model().BBox(df_cube.Pos{}, w)
		for _, bb := range bbs {
			if bb.Width() != 1 || bb.Height() != 1 || bb.Length() != 1 {
				return false
			}
		}

		return true
	}
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
