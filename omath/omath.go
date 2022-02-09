package omath

import (
	"math"

	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
)

// Vec32To64 converts a 32 bit vector to a 64 bit one.
func Vec32To64(vec3 mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(vec3[0]), float64(vec3[1]), float64(vec3[2])}
}

// Round will round a number to a given precision.
func Round(val float64, precision int) float64 {
	p := math.Pow10(precision)
	value := float64(int(val*p)) / p
	return value
}

// AABBVectorDistance calculates the distance between an AABB and a vector.
func AABBVectorDistance(a physics.AABB, v mgl64.Vec3) float64 {
	x, y, z := math.Max(a.Min().X()-v.X(), math.Max(0, v.X()-a.Max().X())),
		math.Max(a.Min().Y()-v.Y(), math.Max(0, v.Y()-a.Max().Y())),
		math.Max(a.Min().Z()-v.Z(), math.Max(0, v.Z()-a.Max().Z()))
	return math.Sqrt((x * x) + (y * y) + (z * z))
}

// DirectionVectorFromValues returns a direction vector from the given yaw and pitch values.
func DirectionVectorFromValues(yaw, pitch float64) mgl64.Vec3 {
	y := -math.Sin(pitch * (math.Pi / 180))
	xz := math.Cos(pitch * (math.Pi / 180))
	x := -xz * math.Sin(yaw*(math.Pi/180))
	z := xz * math.Cos(yaw*(math.Pi/180))
	return mgl64.Vec3{x, y, z}.Normalize()
}
