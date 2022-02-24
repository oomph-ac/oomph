package minecraft

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
	"math"
)

// ClampFloat clamps a float between the provided min and max values.
func ClampFloat(num, min, max float64) float64 {
	if num < min {
		return min
	}
	return math.Min(num, max)
}

// Vec32ToCubePos converts a mgl32.Vec3 to a cube.Pos
func Vec32ToCubePos(vec mgl32.Vec3) cube.Pos {
	return cube.Pos{int(vec.X()), int(vec.Y()), int(vec.Z())}
}
