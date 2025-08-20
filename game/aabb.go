package game

import (
	"github.com/chewxy/math32"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
)

type clipCollideResult struct {
	depenetratingAxis     int
	penetration           float32
	clippedVelocity       mgl32.Vec3
	depenetratingVelocity mgl32.Vec3
}

func BBClipCollide(this, c cube.BBox, vel mgl32.Vec3, oneWay bool, penetration *mgl32.Vec3) mgl32.Vec3 {
	result := doBBClipCollide(this, c, vel)
	if penetration != nil && penetration[result.depenetratingAxis] < result.penetration {
		penetration[result.depenetratingAxis] = result.penetration
	}

	if oneWay {
		return result.clippedVelocity
	}
	return result.depenetratingVelocity
}

func doBBClipCollide(stationary, moving cube.BBox, velocity mgl32.Vec3) (result clipCollideResult) {
	result.clippedVelocity = velocity
	result.depenetratingVelocity = velocity

	if BBHasZeroVolume(stationary) {
		return
	}

	axisPenetrations := [3]float32{}
	axisPenetrationsSigned := [3]float32{}
	normalDirs := [3]float32{}
	seperatingAxes, seperatingAxis := 0, 0
	resultPenetration := float32(math32.MaxFloat32 - 1)

	for i := range 3 {
		minPenetration := moving.Max()[i] - stationary.Min()[i]
		maxPenetration := stationary.Max()[i] - moving.Min()[i]

		if math32.Abs(minPenetration) <= 1e-7 {
			minPenetration = 0
		}
		if math32.Abs(maxPenetration) <= 1e-7 {
			maxPenetration = 0
		}

		minPositive := math32.Max(0, minPenetration)
		maxPositive := math32.Max(0, maxPenetration)

		if minPositive == 0 {
			axisPenetrations[i] = 0
			axisPenetrationsSigned[i] = minPenetration
			normalDirs[i] = -1
			seperatingAxes++
			seperatingAxis = i
		} else if maxPositive == 0 {
			axisPenetrations[i] = 0
			axisPenetrationsSigned[i] = maxPenetration
			normalDirs[i] = 1
			seperatingAxes++
			seperatingAxis = i
		} else if minPositive < maxPositive {
			axisPenetrations[i] = minPositive
			axisPenetrationsSigned[i] = minPositive
			normalDirs[i] = -1
		} else {
			axisPenetrations[i] = maxPositive
			axisPenetrationsSigned[i] = maxPositive
			normalDirs[i] = 1
		}

		if seperatingAxes > 1 {
			return
		}
		resultPenetration = math32.Min(resultPenetration, axisPenetrations[i])
	}

	// No separating axes means a collision
	if seperatingAxes == 0 {
		result.penetration = resultPenetration
		bestAxis := 0
		for i := 1; i < 3; i++ {
			if axisPenetrations[i] < axisPenetrations[bestAxis] {
				bestAxis = i
			}
		}

		desiredVelocity := axisPenetrations[bestAxis] * normalDirs[bestAxis]
		if desiredVelocity > 0 {
			result.depenetratingVelocity[bestAxis] = math32.Max(desiredVelocity, velocity[bestAxis])
		} else {
			result.depenetratingVelocity[bestAxis] = math32.Min(desiredVelocity, velocity[bestAxis])
		}
		result.depenetratingAxis = bestAxis
		return
	}

	sweptPenetration := axisPenetrationsSigned[seperatingAxis] - (normalDirs[seperatingAxis] * velocity[seperatingAxis])
	if sweptPenetration <= 0 {
		return
	}

	resolvedVelocity := axisPenetrationsSigned[seperatingAxis] * normalDirs[seperatingAxis]
	result.clippedVelocity[seperatingAxis] = resolvedVelocity
	result.depenetratingVelocity[seperatingAxis] = resolvedVelocity
	return
}

func BBHasZeroVolume(bb cube.BBox) bool {
	return bb.Min() == bb.Max()
}

func ClosestPointInLineToPoint(start, end, point mgl32.Vec3) mgl32.Vec3 {
	line := end.Sub(start)
	if line.LenSqr() <= 1e-4 {
		return start
	}

	t := (point.Sub(start)).Dot(line) / line.LenSqr()
	// Clamp to stay on the line segment
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	return start.Add(line.Mul(t))
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

	if !hit1 && !hit2 {
		if point1 == point2 || point1.Y() == point2.Y() || (point1.X() == point2.X() && point1.Z() == point2.Z()) {
			return mgl32.Vec3{}, false
		}
	} else if hit1 {
		return point1, true
	} else if hit2 {
		return point2, true
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
