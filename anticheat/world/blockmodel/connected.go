package blockmodel

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/world"
)

type Fence struct {
	Wood bool
}

func (f Fence) BBox(pos cube.Pos, src world.BlockSource) (boxes []cube.BBox) {
	const inset = 0.375

	west, east := f.connects(pos, cube.FaceWest, src), f.connects(pos, cube.FaceEast, src)
	if west || east {
		box := cube.Box(0, 0, 0, 1, 1.5, 1).Stretch(cube.Z, -inset)
		if !west {
			box = box.ExtendTowards(cube.FaceWest, -inset)
		} else if !east {
			box = box.ExtendTowards(cube.FaceEast, -inset)
		}
		boxes = append(boxes, box)
	}

	north, south := f.connects(pos, cube.FaceNorth, src), f.connects(pos, cube.FaceSouth, src)
	if north || south {
		box := cube.Box(0, 0, 0, 1, 1.5, 1).Stretch(cube.X, -inset)
		if !north {
			box = box.ExtendTowards(cube.FaceNorth, -inset)
		} else if !south {
			box = box.ExtendTowards(cube.FaceSouth, -inset)
		}
		boxes = append(boxes, box)
	}

	if len(boxes) == 0 {
		boxes = append(boxes, cube.Box(inset, 0, inset, 1-inset, 1.5, 1-inset))
	}
	return boxes
}

func (f Fence) FaceSolid(_ cube.Pos, face cube.Face, _ world.BlockSource) bool {
	return face == cube.FaceDown || face == cube.FaceUp
}

func (f Fence) connects(pos cube.Pos, face cube.Face, src world.BlockSource) bool {
	sidePos := pos.Side(face)
	sideModel := src.Block(sidePos).Model()
	if fence, ok := sideModel.(model.Fence); ok && fence.Wood == f.Wood {
		return true
	}
	if sideModel.FaceSolid(sidePos, face.Opposite(), src) {
		return true
	}
	_, ok := sideModel.(model.FenceGate)
	return ok
}

type Thin struct{}

func (t Thin) BBox(pos cube.Pos, src world.BlockSource) (boxes []cube.BBox) {
	const widthInset = 7.0 / 16.0
	const endInset = 0.5

	west, east := t.connects(pos, cube.FaceWest, src), t.connects(pos, cube.FaceEast, src)
	if west || east {
		box := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.Z, -widthInset)
		if !west {
			box = box.ExtendTowards(cube.FaceWest, -endInset)
		} else if !east {
			box = box.ExtendTowards(cube.FaceEast, -endInset)
		}
		boxes = append(boxes, box)
	}

	north, south := t.connects(pos, cube.FaceNorth, src), t.connects(pos, cube.FaceSouth, src)
	if north || south {
		box := cube.Box(0, 0, 0, 1, 1, 1).Stretch(cube.X, -widthInset)
		if !north {
			box = box.ExtendTowards(cube.FaceNorth, -endInset)
		} else if !south {
			box = box.ExtendTowards(cube.FaceSouth, -endInset)
		}
		boxes = append(boxes, box)
	}

	if len(boxes) == 0 {
		boxes = append(boxes, cube.Box(widthInset, 0, widthInset, 1-widthInset, 1, 1-widthInset))
	}
	return boxes
}

func (Thin) FaceSolid(_ cube.Pos, face cube.Face, _ world.BlockSource) bool {
	return face == cube.FaceDown
}

func (Thin) connects(pos cube.Pos, face cube.Face, src world.BlockSource) bool {
	sidePos := pos.Side(face)
	sideModel := src.Block(sidePos).Model()
	_, thin := sideModel.(model.Thin)
	_, wall := sideModel.(model.Wall)
	return thin || wall || sideModel.FaceSolid(sidePos, face.Opposite(), src)
}
