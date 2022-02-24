package game

import (
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/go-gl/mathgl/mgl64"
	"math"
)

// AABBFromDimensions returns a bounding box from the given dimensions.
func AABBFromDimensions(width, height float64) physics.AABB {
	h := width / 2
	return physics.NewAABB(
		mgl64.Vec3{-h, 0, -h},
		mgl64.Vec3{h, height, h},
	)
}

// AABBVectorDistance calculates the distance between an AABB and a vector.
func AABBVectorDistance(a physics.AABB, v mgl64.Vec3) float64 {
	x, y, z := math.Max(a.Min().X()-v.X(), math.Max(0, v.X()-a.Max().X())),
		math.Max(a.Min().Y()-v.Y(), math.Max(0, v.Y()-a.Max().Y())),
		math.Max(a.Min().Z()-v.Z(), math.Max(0, v.Z()-a.Max().Z()))
	return math.Sqrt((x * x) + (y * y) + (z * z))
}
