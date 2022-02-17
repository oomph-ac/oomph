package utils

import (
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	omodel "github.com/justtaldevelops/oomph/utils/model"
)

// BlockClimable returns whether the given block is climable.
func BlockClimable(b world.Block) bool {
	name, _ := b.EncodeBlock()
	return name == "minecraft:ladder" || name == "minecraft:vine"
}

type CheckBlock struct {
	min, max  mgl64.Vec3
	curr      world.Block
	w         HasWorld
	epsilonXZ float64
	epsilonY  float64
	first     bool
}

type HasWorld interface {
	// Block reads a block from the position passed. If a chunk is not yet loaded at that position, it will return air.
	Block(pos cube.Pos) world.Block
	AABB() physics.AABB
}

func DefaultCheckBlockSettings(aabb physics.AABB, w HasWorld) CheckBlock {
	aabb = aabb.Grow(1)
	return CheckBlock{
		min:       aabb.Min(),
		max:       aabb.Max(),
		curr:      w.Block(cube.PosFromVec3(aabb.Min())),
		w:         w,
		epsilonXZ: 1,
		epsilonY:  1,
	}
}

func DefaultCheckBlockSettingsWithFirst(aabb physics.AABB, w HasWorld) CheckBlock {
	d := DefaultCheckBlockSettings(aabb, w)
	d.first = true
	return d
}

func (c CheckBlock) SearchAll() (blocks []world.Block) {
	if c.first {
		return []world.Block{c.curr}
	}
	for x := c.min.X(); x < c.max.X(); x += c.epsilonXZ {
		for y := c.min.Y(); x < c.max.Y(); y += c.epsilonY {
			for z := c.min.Z(); x < c.max.Z(); z += c.epsilonXZ {
				blocks = append(blocks, c.w.Block(cube.Pos{int(x), int(y), int(z)}))
			}
		}
	}
	return
}

func (c CheckBlock) SearchSolid() (blocks []world.Block) {
	if (c.curr == (block.Air{}) || !CanPassThroughBlock(c.curr, cube.PosFromVec3(c.min), c.w)) && c.first {
		return []world.Block{c.curr}
	}
	for x := c.min.X(); x < c.max.X(); x += c.epsilonXZ {
		for y := c.min.Y(); x < c.max.Y(); y += c.epsilonY {
			for z := c.min.Z(); x < c.max.Z(); z += c.epsilonXZ {
				pos := cube.Pos{int(x), int(y), int(z)}
				b := c.w.Block(pos)
				if (c.curr == (block.Air{})) || !CanPassThroughBlock(b, pos, c.w) {
					if c.first {
						return []world.Block{b}
					}
				}
				blocks = append(blocks, b)
			}
		}
	}
	return
}

func (c CheckBlock) SearchTransparent() (blocks []world.Block) {
	if c.first {
		_, ok := c.curr.(block.EntityInsider)
		if ok {
			return []world.Block{c.curr}
		}
	}
	for x := c.min.X(); x < c.max.X(); x += c.epsilonXZ {
		for y := c.min.Y(); x < c.max.Y(); y += c.epsilonY {
			for z := c.min.Z(); x < c.max.Z(); z += c.epsilonXZ {
				b := c.w.Block(cube.Pos{int(x), int(y), int(z)})
				if _, ok := b.(block.EntityInsider); ok {
					if c.first {
						return []world.Block{b}
					}
					blocks = append(blocks, b)
				}
			}
		}
	}
	return
}

func GetCollisionBBList(aabb physics.AABB, w HasWorld) (list []physics.AABB) {
	cloneBB := aabb.Grow(1)
	min, max := cloneBB.Min(), cloneBB.Max()
	for z := math.Floor(min.Z()); z <= math.Ceil(max.Z()); z++ {
		for x := math.Floor(min.X()); x <= math.Ceil(max.X()); x++ {
			for y := math.Floor(min.Y()); y <= math.Ceil(max.Y()); y++ {
				pos := cube.Pos{int(x), int(y), int(z)}
				b := w.Block(pos)
				if !CanPassThroughBlock(b, pos, w) {
					for _, bb := range GetAABBS(b, pos, w) {
						bb = physics.NewAABB(
							pos.Vec3().Sub(mgl64.Vec3{bb.Width(), 0, bb.Width()}),
							pos.Vec3().Add(mgl64.Vec3{bb.Width(), bb.Height(), bb.Width()}),
						)
						if bb.IntersectsWith(aabb) {
							list = append(list, bb)
						}
					}
				}
			}
		}
	}
	return
}

func CanPassThroughBlock(b world.Block, pos cube.Pos, w HasWorld) bool {
	translatedPos := mgl64.Vec3{float64(pos.X()), float64(pos.Y()), float64(pos.Z())}
	for _, bb := range GetAABBS(b, pos, w) {
		if bb.Grow(0.05).Translate(translatedPos).IntersectsWith(w.AABB().Translate(translatedPos)) {
			return true
		}
	}
	return false
}

// GetAABBS is a hack to get use models that use the passed world parameter.
func GetAABBS(b world.Block, pos cube.Pos, w HasWorld) []physics.AABB {
	switch m := b.Model().(type) {
	case model.Stair:
		return omodel.Stair{
			Stair: model.Stair{
				Facing:     m.Facing,
				UpsideDown: m.UpsideDown,
			},
		}.AABB(pos, w)
	case model.Fence:
		return omodel.Fence{
			Fence: model.Fence{
				Wood: m.Wood,
			},
		}.AABB(pos, w)
	case model.Thin:
		return omodel.Thin{
			Thin: model.Thin{},
		}.AABB(pos, w)
	}
	return b.Model().AABB(pos, nil)
}
