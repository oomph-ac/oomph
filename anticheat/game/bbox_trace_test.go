package game

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
)

func TestBBoxIntercept(t *testing.T) {
	box := cube.Box32(0, 0, 0, 1, 1, 1)
	tests := []struct {
		name       string
		start, end mgl32.Vec3
		position   mgl32.Vec3
		face       cube.Face
	}{
		{name: "west", start: mgl32.Vec3{-1, 0.5, 0.5}, end: mgl32.Vec3{2, 0.5, 0.5}, position: mgl32.Vec3{0, 0.5, 0.5}, face: cube.FaceWest},
		{name: "east", start: mgl32.Vec3{2, 0.5, 0.5}, end: mgl32.Vec3{-1, 0.5, 0.5}, position: mgl32.Vec3{1, 0.5, 0.5}, face: cube.FaceEast},
		{name: "down", start: mgl32.Vec3{0.5, -1, 0.5}, end: mgl32.Vec3{0.5, 2, 0.5}, position: mgl32.Vec3{0.5, 0, 0.5}, face: cube.FaceDown},
		{name: "up", start: mgl32.Vec3{0.5, 2, 0.5}, end: mgl32.Vec3{0.5, -1, 0.5}, position: mgl32.Vec3{0.5, 1, 0.5}, face: cube.FaceUp},
		{name: "north", start: mgl32.Vec3{0.5, 0.5, -1}, end: mgl32.Vec3{0.5, 0.5, 2}, position: mgl32.Vec3{0.5, 0.5, 0}, face: cube.FaceNorth},
		{name: "south", start: mgl32.Vec3{0.5, 0.5, 2}, end: mgl32.Vec3{0.5, 0.5, -1}, position: mgl32.Vec3{0.5, 0.5, 1}, face: cube.FaceSouth},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, ok := BBoxIntercept(box, test.start, test.end)
			if !ok {
				t.Fatal("BBoxIntercept() did not report an intersection")
			}
			if got := result.BBox(); got != box {
				t.Fatalf("BBoxIntercept().BBox() = %v, want %v", got, box)
			}
			if got := result.Position(); got != test.position {
				t.Fatalf("BBoxIntercept().Position() = %v, want %v", got, test.position)
			}
			if got := result.Face(); got != test.face {
				t.Fatalf("BBoxIntercept().Face() = %v, want %v", got, test.face)
			}
		})
	}
}

func TestBlockPosConversions(t *testing.T) {
	pos := BlockPosFromVec3(mgl32.Vec3{-0.1, 2.9, 3})
	if got, want := pos, (cube.Pos{-1, 2, 3}); got != want {
		t.Fatalf("BlockPosFromVec3() = %v, want %v", got, want)
	}
	if got, want := BlockPosVec3(pos), (mgl32.Vec3{-1, 2, 3}); got != want {
		t.Fatalf("BlockPosVec3() = %v, want %v", got, want)
	}
}
