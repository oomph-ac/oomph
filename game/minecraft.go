package game

import (
	"github.com/chewxy/math32"
	"github.com/go-gl/mathgl/mgl32"
)

// sinTable ...
var sinTable []float32

// init initializes the sinTable.
func init() {
	for i := float32(0.0); i < 65536; i++ {
		sinTable = append(sinTable, math32.Sin(i*math32.Pi*2/65536))
	}
}

// MCSin returns the Minecraft sin of the given angle.
func MCSin(val float32) float32 {
	return sinTable[uint16(val*10430.378)&65535]
}

// MCCos returns the Minecraft cos of the given angle.
func MCCos(val float32) float32 {
	return sinTable[uint16(val*10430.378+16384.0)&65535]
}

// ClampFloat clamp the given value to the given range.
func ClampFloat(num, min, max float32) float32 {
	if num < min {
		return min
	}
	return math32.Min(num, max)
}

// GetRotationToPoint returns the yaw/pitch needed to be aiming at a certain point.
func GetRotationToPoint(origin, target mgl32.Vec3) mgl32.Vec2 {
	hz := math32.Sqrt(math32.Pow(target.X()-origin.X(), 2) + math32.Pow(target.Z()-origin.Z(), 2))
	v := target.Y() - origin.Y()

	pitch := -math32.Atan2(v, hz) / math32.Pi * 180
	xDist, zDist := target.X()-origin.X(), target.Z()-origin.Z()
	yaw := math32.Atan2(zDist, xDist)/math32.Pi*180 - 90
	if yaw <= -180 {
		yaw += 360
	}

	return mgl32.Vec2{yaw, pitch}
}

func AngleToPoint(
	origin,
	target,
	rotation mgl32.Vec3,
) mgl32.Vec2 {
	diff := target.Sub(origin)
	yaw := (math32.Atan2(diff[2], diff[0]) * 180 / math32.Pi) - 90
	pitch := math32.Atan2(diff[1], math32.Sqrt(diff[0]*diff[0]+diff[2]*diff[2])) * 180 / math32.Pi
	if yaw < -180 {
		yaw += 360
	} else if yaw > 180 {
		yaw -= 360
	}
	yawDiff := yaw - rotation[2]
	pitchDiff := pitch - rotation[0]
	return mgl32.Vec2{yawDiff, pitchDiff}
}
