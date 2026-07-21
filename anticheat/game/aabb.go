package game

import (
	"github.com/chewxy/math32"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
)

func ClosestPointInLineToPoint(origin, end mgl32.Vec3, point mgl32.Vec3) mgl32.Vec3 {
	line := end.Sub(origin)
	if line.LenSqr() <= 1e-4 {
		return origin
	}

	t := (point.Sub(origin)).Dot(line) / line.LenSqr()
	// Clamp to stay on the line segment
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	return origin.Add(line.Mul(t))
}

// ClosestPointToBBox returns the shortest point from a given origin to a given bounding box.
func ClosestPointToBBox(origin mgl32.Vec3, bb cube.BBox) mgl32.Vec3 {
	var shortest mgl32.Vec3

	if origin.X() < bb.Min().X() {
		shortest[0] = bb.Min().X()
	} else if origin.X() > bb.Max().X() {
		shortest[0] = bb.Max().X()
	} else {
		shortest[0] = origin.X()
	}

	if origin.Y() < bb.Min().Y() {
		shortest[1] = bb.Min().Y()
	} else if origin.Y() > bb.Max().Y() {
		shortest[1] = bb.Max().Y()
	} else {
		shortest[1] = origin.Y()
	}

	if origin.Z() < bb.Min().Z() {
		shortest[2] = bb.Min().Z()
	} else if origin.Z() > bb.Max().Z() {
		shortest[2] = bb.Max().Z()
	} else {
		shortest[2] = origin.Z()
	}

	return shortest
}

// ClosestPointToBBoxDirectional returns the shortest point from a given origin to a given bounding box, in a given direction.
func ClosestPointToBBoxDirectional(origin, startLook, endLook mgl32.Vec3, bb cube.BBox, distance float32) (mgl32.Vec3, bool) {
	point1 := origin.Add(startLook.Mul(distance))
	point2 := origin.Add(endLook.Mul(distance))

	rayResult1, hit1 := trace.BBoxIntercept(bb, origin, point1)
	if hit1 {
		point1 = rayResult1.Position()
	} else {
		point1 = ClosestPointToBBox(point1, bb)
	}

	rayResult2, hit2 := trace.BBoxIntercept(bb, origin, point2)
	if hit2 {
		point2 = rayResult2.Position()
	} else {
		point2 = ClosestPointToBBox(point2, bb)
	}

	if point1 == point2 {
		if !hit1 {
			if !hit2 {
				// Here, there is no possible way that any point between the two rays can intersect with the bounding box.
				return mgl32.Vec3{}, false
			}
			return point2, true
		}
		return point1, true
	}

	possibleBB := cube.Box(point1.X(), point1.Y(), point1.Z(), point2.X(), point2.Y(), point2.Z())
	return ClosestPointToBBox(origin, possibleBB), true
}

// AABBFromDFBox converts a dragonfly bounding box to a float32-cube bounding box.
func DFBoxToCubeBox(b df_cube.BBox) cube.BBox {
	return cube.Box(
		float32(b.Min().X()), float32(b.Min().Y()), float32(b.Min().Z()),
		float32(b.Max().X()), float32(b.Max().Y()), float32(b.Max().Z()),
	)
}

// CubeBoxToDFBox converts a float32-cube bounding box to a dragonfly bounding box.
func CubeBoxToDFBox(b cube.BBox) df_cube.BBox {
	return df_cube.Box(
		float64(b.Min().X()), float64(b.Min().Y()), float64(b.Min().Z()),
		float64(b.Max().X()), float64(b.Max().Y()), float64(b.Max().Z()),
	)
}

// AABBFromDimensions returns a bounding box from the given dimensions.
func AABBFromDimensions(width, height float32) cube.BBox {
	h := width / 2
	return cube.Box(
		-h, 0, -h,
		h, height, h,
	)
}

// AABBVectorDistance calculates the distance between an AABB and a vector.
func AABBVectorDistance(a cube.BBox, v mgl32.Vec3) float32 {
	x := math32.Max(a.Min().X()-v.X(), math32.Max(0, v.X()-a.Max().X()))
	y := math32.Max(a.Min().Y()-v.Y(), math32.Max(0, v.Y()-a.Max().Y()))
	z := math32.Max(a.Min().Z()-v.Z(), math32.Max(0, v.Z()-a.Max().Z()))

	dist := math32.Sqrt(math32.Pow(x, 2) + math32.Pow(y, 2) + math32.Pow(z, 2))
	if dist == mgl32.NaN {
		dist = 0
	}

	return dist
}

// BBoxCenter gets the center of a bounding box.
func BBoxCenter(bb cube.BBox) mgl32.Vec3 {
	return bb.Min().Add(bb.Max()).Mul(0.5)
}

// BBoxPoints returns all the side points of a given bounding box.
func BBoxPoints(bb cube.BBox) []mgl32.Vec3 {
	min := bb.Min()
	max := bb.Max()

	return []mgl32.Vec3{
		min,                         // 0: min
		{min.X(), min.Y(), max.Z()}, // 1: min, max.z
		{min.X(), max.Y(), min.Z()}, // 2: max.y, min.z
		{min.X(), max.Y(), max.Z()}, // 3: max.y, max.z
		{max.X(), min.Y(), min.Z()}, // 4: max.x, min.z
		{max.X(), min.Y(), max.Z()}, // 5: max.x, max.z
		{max.X(), max.Y(), min.Z()}, // 6: max.x, max.y, min.z
		max,                         // 7: max
		BBoxCenter(bb),              // 8: center
	}
}
