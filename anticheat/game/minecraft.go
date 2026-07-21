package game

import (
	"github.com/chewxy/math32"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	MaxBlockInteractionDistance float32 = 6.0
)

// ClampFloat clamp the given value to the given range.
func ClampFloat(num, min, max float32) float32 {
	if num < min {
		return min
	}
	return math32.Min(num, max)
}

// RotationToPoint returns the yaw/pitch needed to be aiming at a certain point.
func RotationToPoint(origin, target mgl32.Vec3) mgl32.Vec2 {
	diff := target.Sub(origin)
	yaw := (math32.Atan2(diff[2], diff[0]) * 180 / math32.Pi) - 90
	pitch := math32.Atan2(diff[1], math32.Sqrt(diff[0]*diff[0]+diff[2]*diff[2])) * 180 / math32.Pi
	if yaw < -180 {
		yaw += 360
	} else if yaw > 180 {
		yaw -= 360
	}
	return mgl32.Vec2{yaw, -pitch}
}

func AngleToPoint(
	origin,
	target,
	rotation mgl32.Vec3,
) mgl32.Vec2 {
	rot := RotationToPoint(origin, target)
	yawDiff := rot[0] - rotation[2]
	pitchDiff := rot[1] - rotation[0]
	return mgl32.Vec2{WrapYawDelta(yawDiff), pitchDiff}
}
