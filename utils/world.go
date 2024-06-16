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
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	oomph_world "github.com/oomph-ac/oomph/world"
)

const fenceInset = 0.5 - (0.25 / 2)

const (
	noCorner = iota
	cornerRightInner
	cornerLeftInner
	cornerRightOuter
	cornerLeftOuter
)

type CollisionType byte

const (
	CollisionX CollisionType = iota
	CollisionY
	CollisionZ
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
		b := w.GetBlock(pos.Side(f))
		if _, isAir := b.(block.Air); isAir {
			continue
		}

		sblocks[f] = b
	}

	switch BlockName(b) {
	case "minecraft:prismarine_stairs", "minecraft:dark_prismarine_stairs", "minecraft:prismarine_bricks_stairs", "minecraft:granite_stairs",
		"minecraft:diorite_stairs", "minecraft:andesite_stairs", "minecraft:polished_granite_stairs", "minecraft:polished_diorite_stairs", "minecraft:polished_andesite_stairs",
		"minecraft:mossy_stone_brick_stairs", "minecraft:smooth_red_sandstone_stairs", "minecraft:smooth_sandstone_stairs", "minecraft:end_brick_stairs",
		"minecraft:mossy_cobblestone_stairs", "minecraft:normal_stone_stairs", "minecraft:red_nether_brick_stairs", "minecraft:smooth_quartz_stairs",
		"minecraft:oak_stairs", "minecraft:stone_stairs", "minecraft:brick_stairs", "minecraft:stone_brick_stairs", "minecraft:nether_brick_stairs",
		"minecraft:sandstone_stairs", "minecraft:spruce_stairs", "minecraft:birch_stairs", "minecraft:jungle_stairs", "minecraft:quartz_stairs",
		"minecraft:acacia_stairs", "minecraft:dark_oak_stairs", "minecraft:red_sandstone_stairs", "minecraft:purpur_stairs", "minecraft:mangrove_stairs",
		"minecraft:deepslate_brick_stairs":
		stair := b.Model().(model.Stair)
		upsideDown := stair.UpsideDown
		direction := cube.Direction(stair.Facing)
		face := direction.Face()
		oppositeFace := direction.Opposite().Face()

		var bbs = []cube.BBox{}
		if !upsideDown {
			bbs = append(bbs, cube.Box(0, 0, 0, 1, 0.5, 1))
		} else {
			bbs = append(bbs, cube.Box(0, 0.5, 0, 1, 1, 1))
		}

		// HACK: Since EncodeBlock() will sometimes return the wrong direction due to the world being passed
		// to it being nil, we will brute force possible directions to see if the stair could be a corner.
		possibleDirection := cube.Direction(-1)
		possibleStairType := uint8(0)
		for _, dir := range []cube.Direction{direction.RotateLeft(), direction.RotateRight()} {
			sType := StairCornerType(dir, upsideDown, sblocks)

			// We only want to apply this possibility if there is a neigboring stair block in the direction.
			bl, ok := sblocks[dir.Face()]
			if !ok {
				continue
			}

			// If the block isn't a stair, we don't want to apply this possibility.
			if _, isStair := bl.Model().(model.Stair); !isStair {
				continue
			}

			if possibleDirection != -1 && possibleStairType != cornerLeftOuter && possibleStairType != cornerRightOuter {
				possibleDirection = dir
				possibleStairType = sType
			}
		}

		// HAHAHAHA. FUCK YOU STAIRS! - @ethaniccc
		stairType := StairCornerType(direction, upsideDown, sblocks)
		if stairType == noCorner && possibleDirection != -1 {
			direction = possibleDirection
			face = direction.Face()
			oppositeFace = direction.Opposite().Face()
			stairType = possibleStairType
		}

		if stairType == noCorner || stairType == cornerRightInner || stairType == cornerLeftInner {
			box := cube.Box(0.5, 0.5, 0.5, 0.5, 1, 0.5).
				ExtendTowards(face, 0.5).
				Stretch(direction.RotateRight().Face().Axis(), 0.5)
			bbs = append(bbs, box)
		}

		box := cube.Box(0.5, 0.5, 0.5, 0.5, 1, 0.5)
		switch stairType {
		case cornerRightOuter:
			bbs = append(bbs, box.
				ExtendTowards(face, 0.5).
				ExtendTowards(direction.RotateLeft().Face(), 0.5))
		case cornerLeftOuter:
			bbs = append(bbs, box.
				ExtendTowards(face, 0.5).
				ExtendTowards(direction.RotateRight().Face(), 0.5))
		case cornerRightInner:
			bbs = append(bbs, box.
				ExtendTowards(oppositeFace, 0.5).
				ExtendTowards(direction.RotateRight().Face(), 0.5))
		case cornerLeftInner:
			bbs = append(bbs, box.
				ExtendTowards(oppositeFace, 0.5).
				ExtendTowards(direction.RotateLeft().Face(), 0.5))
		}

		if upsideDown {
			for i := range bbs[1:] {
				bbs[i+1] = bbs[i+1].Translate(mgl32.Vec3{0, -0.5})
			}
		}

		return bbs
	case "minecraft:wooden_pressure_plate", "minecraft:spruce_pressure_plate", "minecraft:birch_pressure_plate",
		"minecraft:jungle_pressure_plate", "minecraft:acacia_pressure_plate", "minecraft:dark_oak_pressure_plate",
		"minecraft:mangrove_pressure_plate", "minecraft:cherry_pressure_plate", "minecraft:crimson_pressure_plate",
		"minecraft:warped_pressure_plate", "minecraft:stone_pressure_plate", "minecraft:light_weighted_pressure_plate",
		"minecraft:heavy_weighted_pressure_plate", "minecraft:polished_blackstone_pressure_plate":
		return []cube.BBox{}
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
	case "minecraft:oak_fence", "minecraft:spruce_fence", "minecraft:birch_fence", "minecraft:jungle_fence",
		"minecraft:acacia_fence", "minecraft:dark_oak_fence", "minecraft:mangrove_fence", "minecraft:cherry_fence",
		"minecraft:crimson_fence", "minecraft:warped_fence":
		var bbs []cube.BBox

		height := float32(1.5)
		connectWest, connectEast, connectNorth, connectSouth := CheckFenceConnections(b, sblocks)
		if connectWest || connectEast {
			bb := cube.Box(0, 0, 0, 1, height, 1).
				Stretch(cube.Z, -fenceInset)

			if !connectWest {
				bb = bb.ExtendTowards(cube.FaceWest, -fenceInset)
			}

			if !connectEast {
				bb = bb.ExtendTowards(cube.FaceEast, -fenceInset)
			}

			bbs = append(bbs, bb)
		}

		if connectNorth || connectSouth {
			bb := cube.Box(0, 0, 0, 1, height, 1).
				Stretch(cube.X, -fenceInset)

			if !connectNorth {
				bb = bb.ExtendTowards(cube.FaceNorth, -fenceInset)
			}

			if !connectSouth {
				bb = bb.ExtendTowards(cube.FaceSouth, -fenceInset)
			}

			bbs = append(bbs, bb)
		}

		if len(bbs) == 0 {
			return []cube.BBox{cube.Box(0, 0, 0, 1, height, 1).
				Stretch(cube.X, -fenceInset).
				Stretch(cube.Z, -fenceInset),
			}
		}

		return bbs
	case "minecraft:iron_bars":
		var bbs []cube.BBox
		connectWest, connectEast, connectNorth, connectSouth := CheckFenceConnections(b, sblocks)
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
	case "minecraft:cobblestone_wall", "minecraft:blackstone_wall", "minecraft:polished_blackstone_wall",
		"minecraft:cobbled_deepslate_wall", "minecraft:polished_blackstone_brick_wall", "minecraft:deepslate_tile_wall",
		"minecraft:mud_brick_wall", "minecraft:polished_deepslate_wall", "minecraft:deepslate_brick_wall":
		var connectedEast bool = false
		var connectedWest bool = false
		var connectedNorth bool = false
		var connectedSouth bool = false

		_, dat := b.EncodeBlock()
		if east, ok := dat["wall_connection_type_east"].(string); ok && east != "none" {
			connectedEast = true
		}
		if west, ok := dat["wall_connection_type_west"].(string); ok && west != "none" {
			connectedWest = true
		}
		if north, ok := dat["wall_connection_type_north"].(string); ok && north != "none" {
			connectedNorth = true
		}
		if south, ok := dat["wall_connection_type_south"].(string); ok && south != "none" {
			connectedSouth = true
		}

		isPost := (dat["wall_post_bit"].(uint8)) > 0
		nonPostCond1 := connectedEast && connectedWest && !connectedNorth && !connectedSouth
		nonPostCond2 := !connectedEast && !connectedWest && connectedNorth && connectedSouth

		inset := float32(0.25)
		if !isPost && (nonPostCond1 || nonPostCond2) {
			inset = float32(0.3125)
		}

		bb := cube.Box(0, 0, 0, 1, 1.5, 1)
		if !connectedNorth {
			bb = bb.ExtendTowards(cube.FaceNorth, -inset)
		}
		if !connectedSouth {
			bb = bb.ExtendTowards(cube.FaceSouth, -inset)
		}
		if !connectedEast {
			bb = bb.ExtendTowards(cube.FaceEast, -inset)
		}
		if !connectedWest {
			bb = bb.ExtendTowards(cube.FaceWest, -inset)
		}

		return []cube.BBox{bb}
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
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)} // HACK.
	case "minecraft:vine", "minecraft:cave_vines", "minecraft:cave_vines_body_with_berries", "minecraft:cave_vines_head_with_berries",
		"minecraft:twisting_vines", "minecraft:weeping_vines":
		return []cube.BBox{}
	case "minecraft:flower_pot":
		return []cube.BBox{cube.Box(0, 0, 0, 13/16.0, 3/8.0, 13/16.0)}
	case "minecraft:black_candle":
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	case "minecraft:tallgrass", "minecraft:fern", "minecraft:large_fern", "minecraft:rose_bush", "minecraft:peony", "minecraft:paeonia":
		return []cube.BBox{}
	case "minecraft:acacia_trapdoor", "minecraft:birch_trapdoor", "minecraft:dark_oak_trapdoor", "minecraft:jungle_trapdoor", "minecraft:spruce_trapdoor",
		"minecraft:trapdoor", "minecraft:iron_trapdoor", "minecraft:wooden_trapdoor", "minecraft:mangrove_trapdoor", "minecraft:cherry_trapdoor":
		model, ok := b.Model().(model.Trapdoor)

		// This is a hack to fix crashes where a trapdoor unregistered by dragonfly is in the world.
		if !ok {
			break
		}

		bb := cube.Box(0, 0, 0, 1, 1, 1)
		trim := float32(0.8175)

		if model.Open {
			return []cube.BBox{bb.ExtendTowards(cube.Face(model.Facing.Face()), -trim)}
		}

		trimFace := cube.FaceDown
		if model.Top {
			trimFace = cube.FaceUp
		}

		return []cube.BBox{bb.ExtendTowards(trimFace, trim)}
	}

	dfBoxes := b.Model().BBox(df_cube.Pos{
		pos.X(), pos.Y(), pos.Z(),
	}, nil)

	var boxes []cube.BBox
	for _, bb := range dfBoxes {
		boxes = append(boxes, game.DFBoxToCubeBox(bb))
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

// StairCornerType returns the corner type of the stair block.
func StairCornerType(currentDirection cube.Direction, upsideDown bool, sblocks map[cube.Face]world.Block) uint8 {
	rotatedFacing := currentDirection.RotateRight()
	// Check if there is a block on the side of the stair.
	if closedSide, ok := sblocks[currentDirection.Face()]; ok {
		// Check if the block on this side is a satir block.
		if closedStair, ok := closedSide.Model().(model.Stair); ok && closedStair.UpsideDown == upsideDown {
			// If the direction of the side stair is equal to the direction of this stair, it's a left outer corner.
			if cube.Direction(closedStair.Facing) == rotatedFacing {
				return cornerLeftOuter
			} else if cube.Direction(closedStair.Facing) == rotatedFacing.Opposite() {
				if side, ok := sblocks[currentDirection.RotateRight().Face()]; ok {
					sideStair, ok := side.Model().(model.Stair)
					if !ok {
						return cornerRightOuter
					}

					if cube.Direction(sideStair.Facing) != currentDirection || sideStair.UpsideDown != upsideDown {
						return cornerRightOuter
					}
					return noCorner
				}
				return noCorner
			}
		}
	}

	openSide, ok := sblocks[currentDirection.Opposite().Face()]
	if !ok {
		return noCorner
	}

	openStair, ok := openSide.Model().(model.Stair)
	if !ok {
		return noCorner
	}

	if openStair.UpsideDown == upsideDown {
		if cube.Direction(openStair.Facing) == rotatedFacing {
			side, ok := sblocks[currentDirection.RotateRight().Face()]
			if !ok {
				return noCorner
			}

			sideStair, ok := side.Model().(model.Stair)
			if !ok || (cube.Direction(sideStair.Facing) != currentDirection || sideStair.UpsideDown != upsideDown) {
				return cornerRightInner
			}
		} else if cube.Direction(openStair.Facing) == rotatedFacing.Opposite() {
			return cornerLeftInner
		}
	}

	return noCorner
}

// Check fence connection checks for connections on the x and z axis the fence may have.
func CheckFenceConnections(b world.Block, sblocks map[cube.Face]world.Block) (bool, bool, bool, bool) {
	// Connections on the X-axis.
	wb, connectWest := sblocks[cube.FaceWest]
	eb, connectEast := sblocks[cube.FaceEast]

	// Check if the block can connect with the fence.
	if connectWest && !FenceConnectionCompatiable(wb) {
		connectWest = false
	}
	if connectEast && !FenceConnectionCompatiable(eb) {
		connectEast = false
	}

	// Connections on the Z-axis.
	nb, connectNorth := sblocks[cube.FaceNorth]
	sb, connectSouth := sblocks[cube.FaceSouth]

	// Check if the block can connect with the fence.
	if connectNorth && !FenceConnectionCompatiable(nb) {
		connectNorth = false
	}
	if connectSouth && !FenceConnectionCompatiable(sb) {
		connectSouth = false
	}

	return connectWest, connectEast, connectNorth, connectSouth
}

// FenceConnectionCompatiable returns true if the given block is compatiable to conenct to a fence.
func FenceConnectionCompatiable(b world.Block) bool {
	if _, isFence := b.Model().(model.Fence); isFence {
		return true
	}

	if _, isFenceGate := b.Model().(model.FenceGate); isFenceGate {
		return true
	}

	n := BlockName(b)
	switch n {
	case "minecraft:azalea_leaves", "minecraft:azalea_leaves_flowered", "minecraft:cherry_leaves", "minecraft:leaves",
		"minecraft:leaves2", "minecraft:mangrove_leaves":
		return false
	default:
		if IsWall(n) {
			return false
		}

		// Assume for now only blocks that are 1x1x1 are compatiable for connections. True for majority of blocks.
		// FIXME: Some non-full cube blocks can still make a connection w/ walls.
		bbs := b.Model().BBox(df_cube.Pos{}, nil)
		for _, bb := range bbs {
			if bb.Width() != 1 || bb.Height() != 1 || bb.Length() != 1 {
				return false
			}
		}

		return true
	}
}

// WallConnectionCompatiable returns true if the given block is compatiable to conenct to a wall.
func WallConnectionCompatiable(b world.Block) bool {
	if _, isWall := b.Model().(model.Wall); isWall {
		return true
	}

	n := BlockName(b)
	switch n {
	case "minecraft:azalea_leaves", "minecraft:azalea_leaves_flowered", "minecraft:cherry_leaves", "minecraft:leaves",
		"minecraft:leaves2", "minecraft:mangrove_leaves":
		return false
	default:
		if IsFence(b) {
			return false
		}

		// Assume for now only blocks that are 1x1x1 are compatiable for connections. True for majority of blocks.
		// FIXME: Some non-full cube blocks can still make a connection w/ walls.
		bbs := b.Model().BBox(df_cube.Pos{}, nil)
		for _, bb := range bbs {
			if bb.Width() != 1 || bb.Height() != 1 || bb.Length() != 1 {
				return false
			}
		}

		return true
	}
}

// IsBBFullCube returns true if the bounding box is 1x1x1.
func IsBBFullCube(bb cube.BBox) bool {
	return bb.Width() == 1 && bb.Height() == 1 && bb.Length() == 1
}

// IsBlockFullCube returns true if at least one of the bounding boxes of the block is 1x1x1.
func IsBlockFullCube(pos cube.Pos, w *oomph_world.World) bool {
	block := w.GetBlock(cube.Pos(pos))
	for _, bb := range BlockBoxes(block, pos, w) {
		if !IsBBFullCube(bb) {
			continue
		}

		return true
	}

	return false
}

// IsBlockOpenSpace returns true if the blocks at the given position and above the given position are not considered "full cubes".
func IsBlockOpenSpace(pos cube.Pos, w *oomph_world.World) bool {
	return !IsBlockFullCube(pos, w) && !IsBlockFullCube(pos.Side(cube.FaceUp), w)
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
				b := w.GetBlock(pos)
				if _, isAir := b.(block.Air); !includeAir && isAir {
					b = nil
					continue
				}

				// If the hash is MaxUint64, then the block is unknown to dragonfly.
				if !includeUnknown && b.Hash() == math.MaxUint64 {
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
				block := w.GetBlock(pos)

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

// BoxesIntersect returns wether or not the first given box intersects with
// the other boxes in the given list.
func BoxesIntersect(bb cube.BBox, boxes []cube.BBox, bpos mgl32.Vec3) bool {
	for _, box := range boxes {
		if box.Translate(bpos).IntersectsWith(bb) {
			return false
		}
	}

	return true
}

// DoBoxCollision does the block collision for the given collision type.
func DoBoxCollision(c CollisionType, bb cube.BBox, boxes []cube.BBox, delta float32) (cube.BBox, float32) {
	for _, box := range boxes {
		switch c {
		case CollisionX:
			delta = bb.XOffset(box, delta)
		case CollisionY:
			delta = bb.YOffset(box, delta)
		case CollisionZ:
			delta = bb.ZOffset(box, delta)
		}
	}

	switch c {
	case CollisionX:
		bb = bb.Translate(mgl32.Vec3{delta})
	case CollisionY:
		bb = bb.Translate(mgl32.Vec3{0, delta})
	case CollisionZ:
		bb = bb.Translate(mgl32.Vec3{0, 0, delta})
	}

	return bb, delta
}

// BlockToCubePos converts protocol.BlockPos into cube.Pos
func BlockToCubePos(p [3]int32) cube.Pos {
	return cube.Pos{int(p[0]), int(p[1]), int(p[2])}
}
