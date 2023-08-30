package utils

import (
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

const fenceInset = 0.5 - (0.25 / 2)

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

// BlockBoxes returns the bounding boxes of the given block based on it's name.
func BlockBoxes(b world.Block, pos df_cube.Pos, sblocks map[cube.Face]world.Block) []df_cube.BBox {
	switch BlockName(b) {
	case "minecraft:wooden_pressure_plate", "minecraft:spruce_pressure_plate", "minecraft:birch_pressure_plate",
		"minecraft:jungle_pressure_plate", "minecraft:acacia_pressure_plate", "minecraft:dark_oak_pressure_plate",
		"minecraft:mangrove_pressure_plate", "minecraft:cherry_pressure_plate", "minecraft:crimson_pressure_plate",
		"minecraft:warped_pressure_plate", "minecraft:stone_pressure_plate", "minecraft:light_weighted_pressure_plate",
		"minecraft:heavy_weighted_pressure_plate", "minecraft:polished_blackstone_pressure_plate":
		return []df_cube.BBox{}
	case "minecraft:trip_wire":
		return []df_cube.BBox{}
	case "minecraft:web":
		return []df_cube.BBox{df_cube.Box(0, 0, 0, 1, 1, 1)}
	case "minecraft:bed":
		return []df_cube.BBox{df_cube.Box(0, 0, 0, 1, 1.0-(7.0/16.0), 1)}
	case "minecraft:waterlily":
		return []df_cube.BBox{df_cube.Box(0, 0, 0, 1, 1.0/64.0, 1)}
	case "minecraft:oak_fence", "minecraft:spruce_fence", "minecraft:birch_fence", "minecraft:jungle_fence",
		"minecraft:acacia_fence", "minecraft:dark_oak_fence", "minecraft:mangrove_fence", "minecraft:cherry_fence",
		"minecraft:crimson_fence", "minecraft:warped_fence":
		var bbs []df_cube.BBox

		// Connections on the X-axis.
		_, connectWest := sblocks[cube.FaceWest]
		_, connectEast := sblocks[cube.FaceEast]

		// Connections on the Z-axis.
		_, connectNorth := sblocks[cube.FaceNorth]
		_, connectSouth := sblocks[cube.FaceSouth]

		if connectWest || connectEast {
			bb := df_cube.Box(0, 0, 0, 1, 1.5, 1).
				Stretch(df_cube.Z, -fenceInset)

			if !connectWest {
				bb = bb.ExtendTowards(df_cube.FaceWest, -fenceInset)
			}

			if !connectEast {
				bb = bb.ExtendTowards(df_cube.FaceEast, -fenceInset)
			}

			bbs = append(bbs, bb)
		}

		if connectNorth || connectSouth {
			bb := df_cube.Box(0, 0, 0, 1, 1.5, 1).
				Stretch(df_cube.X, -fenceInset)

			if !connectNorth {
				bb = bb.ExtendTowards(df_cube.FaceNorth, -fenceInset)
			}

			if !connectSouth {
				bb = bb.ExtendTowards(df_cube.FaceSouth, -fenceInset)
			}

			bbs = append(bbs, bb)
		}

		if len(bbs) == 0 {
			return []df_cube.BBox{df_cube.Box(0, 0, 0, 1, 1.5, 1).
				Stretch(df_cube.X, -fenceInset).
				Stretch(df_cube.Z, -fenceInset),
			}
		}

		return bbs
	}

	return b.Model().BBox(pos, nil)
}

// BlockClimbable returns whether the given block is climbable.
func BlockClimbable(b world.Block) bool {
	switch b.(type) {
	case block.Ladder:
		return true
		// TODO: Add vines here.
	}
	return false
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
