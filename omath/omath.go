package omath

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"math"
)

// Vec32to64 ...
func Vec32To64(vec3 mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(vec3[0]), float64(vec3[1]), float64(vec3[2])}
}

// Round ...
func Round(val float64, precision int) float64 {
	p := math.Pow10(precision)
	value := float64(int(val*p)) / p
	return value
}
