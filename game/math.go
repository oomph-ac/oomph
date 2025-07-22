package game

import (
	"math"

	"github.com/chewxy/math32"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
)

// LerpRotation lerps a rotation by the given partial ticks.
func LerpRotation(from, to mgl32.Vec3, partialTicks float32) mgl32.Vec3 {
	delta := to.Sub(from)
	delta[0] = math32.Mod(delta[0], 360.0)
	delta[1] = math32.Mod(delta[1], 360.0)
	delta[2] = math32.Mod(delta[2], 360.0)

	if delta[0] < 0 {
		delta[0] += 360.0
	}
	if delta[1] < 0 {
		delta[1] += 360.0
	}
	if delta[2] < 0 {
		delta[2] += 360.0
	}

	return from.Add(delta)
}

// Round32 will round a float32 to a given precision.
func Round32(val float32, precision int) float32 {
	pwr := math32.Pow(10, float32(precision))
	return math32.Round(val*pwr) / pwr
}

// Round64 will round a float64 to a given precision.
func Round64(val float64, precision int) float64 {
	pwr := math.Pow(10, float64(precision))
	return math.Round(val*pwr) / pwr
}

// Float32ApproxEq determines whether two floating point numbers are close enough to each other
// by a threshold of 1e-5.
func Float32ApproxEq(a, b float32) bool {
	return math32.Abs(a-b) <= 1e-5
}

// AbsInt64 will return the absolute value of an int64.
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

// RoundVec64 will round a 64-bit vector to a given precision.
func RoundVec64(v mgl64.Vec3, p int) mgl64.Vec3 {
	return mgl64.Vec3{Round64(v.X(), p), Round64(v.Y(), p), Round64(v.Z(), p)}
}

// RoundVec32 will round a 32-bit vector to a given precision.
func RoundVec32(v mgl32.Vec3, p int) mgl32.Vec3 {
	return mgl32.Vec3{Round32(v.X(), p), Round32(v.Y(), p), Round32(v.Z(), p)}
}

// DirectionVector returns a direction vector from the given yaw and pitch values.
func DirectionVector(yaw, pitch float32) mgl32.Vec3 {
	yawRad, pitchRad := mgl32.DegToRad(yaw), mgl32.DegToRad(pitch)
	m := math32.Cos(pitchRad)

	return mgl32.Vec3{
		-m * math32.Sin(yawRad),
		-math32.Sin(pitchRad),
		m * math32.Cos(yawRad),
	}
}

// WrapYawDelta ...
func WrapYawDelta(delta float32) float32 {
	if delta > 180 {
		delta -= 360
	} else if delta < -180 {
		delta += 360
	}
	return delta
}

// AbsVec64 will return the given vector, but all the values of it are switched to their absolute values.
func AbsVec64(vec mgl64.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{math.Abs(vec.X()), math.Abs(vec.Y()), math.Abs(vec.Z())}
}

// AbsVec32 will return the given vector, but all the values of it are switched to their absolute values.
func AbsVec32(vec mgl32.Vec3) mgl32.Vec3 {
	return mgl32.Vec3{math32.Abs(vec.X()), math32.Abs(vec.Y()), math32.Abs(vec.Z())}
}

// Vec3HzDistSqr returns the squared horizontal distance in a vector.
func Vec3HzDistSqr(vec3 mgl32.Vec3) float32 {
	return vec3.X()*vec3.X() + vec3.Z()*vec3.Z()
}

func MinVec3(vecs []mgl32.Vec3) mgl32.Vec3 {
	min := mgl32.Vec3{math32.MaxFloat32, math32.MaxFloat32, math32.MaxFloat32}
	for _, v := range vecs {
		if v[0] <= min[0] || v[1] <= min[1] || v[2] <= min[2] {
			min = v
		}
	}

	return min
}

func MaxVec3(vecs []mgl32.Vec3) mgl32.Vec3 {
	max := mgl32.Vec3{-math32.MaxFloat32, -math32.MaxFloat32, -math32.MaxFloat32}
	for _, v := range vecs {
		if v[0] >= max[0] || v[1] >= max[1] || v[2] >= max[2] {
			max = v
		}
	}

	return max
}

// Returns -1 if x < y, 0 if x == y, or 1 if x > y
func PHPSpaceshipOp(x, y float32) float32 {
	if x < y {
		return -1
	} else if x == y {
		return 0
	}

	return 1
}
