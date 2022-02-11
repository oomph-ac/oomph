package omath

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
	"math"
)

var sinTable []float64

func init() {
	for i := float64(0); i < 65536; i++ {
		sinTable = append(sinTable, math.Sin(i*math.Pi*2/65536))
	}
}

func MCSin(val float64) float64 {
	return sinTable[uint16(val*10430.378)&65535]
}

func MCCos(val float64) float64 {
	return sinTable[uint16(val*10430.378+16384.0)&65535]
}

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
