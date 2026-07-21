package game

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// BBoxResult is the result of a ray intersection with a float32 bounding box.
type BBoxResult struct {
	box  cube.BBox32
	pos  mgl32.Vec3
	face cube.Face
}

// NewBBoxResult returns a bounding-box intersection result.
func NewBBoxResult(box cube.BBox32, pos mgl32.Vec3, face cube.Face) BBoxResult {
	return BBoxResult{box: box, pos: pos, face: face}
}

// BBox returns the bounding box that was intersected.
func (r BBoxResult) BBox() cube.BBox32 {
	return r.box
}

// Position returns the intersection position.
func (r BBoxResult) Position() mgl32.Vec3 {
	return r.pos
}

// Face returns the face that was intersected.
func (r BBoxResult) Face() cube.Face {
	return r.face
}

// BBoxIntercept returns the closest intersection between a line segment and a bounding box.
func BBoxIntercept(box cube.BBox32, start, end mgl32.Vec3) (result BBoxResult, ok bool) {
	min, max := box.Min(), box.Max()
	v1 := vec3OnLineWithX(start, end, min[0])
	v2 := vec3OnLineWithX(start, end, max[0])
	v3 := vec3OnLineWithY(start, end, min[1])
	v4 := vec3OnLineWithY(start, end, max[1])
	v5 := vec3OnLineWithZ(start, end, min[2])
	v6 := vec3OnLineWithZ(start, end, max[2])

	if v1 != nil && !box.Vec3WithinYZ(*v1) {
		v1 = nil
	}
	if v2 != nil && !box.Vec3WithinYZ(*v2) {
		v2 = nil
	}
	if v3 != nil && !box.Vec3WithinXZ(*v3) {
		v3 = nil
	}
	if v4 != nil && !box.Vec3WithinXZ(*v4) {
		v4 = nil
	}
	if v5 != nil && !box.Vec3WithinXY(*v5) {
		v5 = nil
	}
	if v6 != nil && !box.Vec3WithinXY(*v6) {
		v6 = nil
	}

	var (
		intersection *mgl32.Vec3
		distance     = float32(math.MaxFloat32)
	)
	for _, candidate := range [...]*mgl32.Vec3{v1, v2, v3, v4, v5, v6} {
		if candidate == nil {
			continue
		}
		if candidateDistance := start.Sub(*candidate).LenSqr(); candidateDistance < distance {
			intersection = candidate
			distance = candidateDistance
		}
	}
	if intersection == nil {
		return BBoxResult{}, false
	}

	var face cube.Face
	switch intersection {
	case v1:
		face = cube.FaceWest
	case v2:
		face = cube.FaceEast
	case v3:
		face = cube.FaceDown
	case v4:
		face = cube.FaceUp
	case v5:
		face = cube.FaceNorth
	case v6:
		face = cube.FaceSouth
	}
	return NewBBoxResult(box, *intersection, face), true
}

func vec3OnLineWithX(a, b mgl32.Vec3, x float32) *mgl32.Vec3 {
	if mgl32.FloatEqual(b[0], a[0]) {
		return nil
	}
	f := (x - a[0]) / (b[0] - a[0])
	if f < 0 || f > 1 {
		return nil
	}
	return &mgl32.Vec3{x, a[1] + (b[1]-a[1])*f, a[2] + (b[2]-a[2])*f}
}

func vec3OnLineWithY(a, b mgl32.Vec3, y float32) *mgl32.Vec3 {
	if mgl32.FloatEqual(a[1], b[1]) {
		return nil
	}
	f := (y - a[1]) / (b[1] - a[1])
	if f < 0 || f > 1 {
		return nil
	}
	return &mgl32.Vec3{a[0] + (b[0]-a[0])*f, y, a[2] + (b[2]-a[2])*f}
}

func vec3OnLineWithZ(a, b mgl32.Vec3, z float32) *mgl32.Vec3 {
	if mgl32.FloatEqual(a[2], b[2]) {
		return nil
	}
	f := (z - a[2]) / (b[2] - a[2])
	if f < 0 || f > 1 {
		return nil
	}
	return &mgl32.Vec3{a[0] + (b[0]-a[0])*f, a[1] + (b[1]-a[1])*f, z}
}
