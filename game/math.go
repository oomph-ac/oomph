package game

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
)

// Round will round a number to a given precision.
func Round(val float64, precision int) float64 {
	pwr := math.Pow(10, float64(precision))
	return math.Round(val*pwr) / pwr
}

func AbsInt64(a int64) int64 {
	if a < 0 {
		a = -a
	}
	return a
}

// Vec32To64 converts a 32-bit vector to a 64-bit one.
func Vec32To64(vec3 mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(vec3[0]), float64(vec3[1]), float64(vec3[2])}
}

// Vec64To32 converts a 64-bit vector to a 32-bit one.
func Vec64To32(vec3 mgl64.Vec3) mgl32.Vec3 {
	return mgl32.Vec3{float32(vec3[0]), float32(vec3[1]), float32(vec3[2])}
}

func RoundVec64(v mgl64.Vec3, p int) mgl64.Vec3 {
	return mgl64.Vec3{Round(v.X(), p), Round(v.Y(), p), Round(v.Z(), p)}
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

// AbsVec64 will return the given vector, but all the values of it are switched to their absolute values.
func AbsVec64(vec mgl64.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{math.Abs(vec.X()), math.Abs(vec.Y()), math.Abs(vec.Z())}
}
