package minecraft

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

// DirectionVector returns a direction vector from the given yaw and pitch values.
func DirectionVector(yaw, pitch float64) mgl64.Vec3 {
	yawRad, pitchRad := mgl64.DegToRad(yaw), mgl64.DegToRad(pitch)
	m := math.Cos(pitchRad)

	return mgl64.Vec3{
		-m * math.Sin(yawRad),
		-math.Sin(pitchRad),
		m * math.Cos(yawRad),
	}
}

// FloorVec64 will floor all the values of a 64 bit vector3 and return a new vector.
func FloorVec64(vec mgl64.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{math.Floor(vec.X()), math.Floor(vec.Y()), math.Floor(vec.Z())}
}

// AbsVec64 will return the given vector, but all the values of it are switched to their absolute values.
func AbsVec64(vec mgl64.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{math.Abs(vec.X()), math.Abs(vec.Y()), math.Abs(vec.Z())}
}
