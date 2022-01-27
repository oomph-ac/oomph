package omath

import (
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"math"
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
	return math.Sqrt(math.Pow(math.Max(a.Min().X()-v.X(), math.Max(0, v.X()-a.Max().X())), 2) + math.Pow(math.Max(a.Min().Y()-v.Y(), math.Max(0, v.Y()-a.Max().Y())), 2) + math.Pow(math.Max(a.Min().Z()-v.Z(), math.Max(0, v.Z()-a.Max().Z())), 2))
}

// DirectionVectorFromValues returns a direction vector from the given yaw and pitch values.
func DirectionVectorFromValues(yaw, pitch float64) mgl64.Vec3 {
	yawRad, pitchRad := mgl64.DegToRad(yaw), mgl64.DegToRad(pitch)
	m := math.Cos(pitchRad)

	return mgl64.Vec3{
		-m * math.Sin(yawRad),
		-math.Sin(pitchRad),
		m * math.Cos(yawRad),
	}
}
