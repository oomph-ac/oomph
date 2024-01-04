package game

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
)

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
func ClosestPointToBBoxDirectional(origin, startLook, endLook mgl32.Vec3, bb cube.BBox, distance float32) mgl32.Vec3 {
	point1 := origin.Add(startLook.Mul(distance))
	point2 := origin.Add(endLook.Mul(distance))

	rayResult, hit := trace.BBoxIntercept(bb, origin, point1)
	if hit {
		point1 = rayResult.Position()
	} else {
		point1 = ClosestPointToBBox(point1, bb)
	}

	rayResult, hit = trace.BBoxIntercept(bb, origin, point2)
	if hit {
		point2 = rayResult.Position()
	} else {
		point2 = ClosestPointToBBox(point2, bb)
	}

	if point1 == point2 {
		return point1
	}

	possibleBB := cube.Box(point1.X(), point1.Y(), point1.Z(), point2.X(), point2.Y(), point2.Z())
	return ClosestPointToBBox(origin, possibleBB)
}
