package bedsim

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

var mcSinTable []float64

func init() {
	mcSinTable = make([]float64, 65536)
	for i := range 65536 {
		mcSinTable[i] = math.Sin(float64(i) * math.Pi * 2 / 65536)
	}
}

// MCSin returns the Minecraft sin of the given angle.
func MCSin(val float64) float64 {
	return mcSinTable[uint16(val*10430.378)&65535]
}

// MCCos returns the Minecraft cos of the given angle.
func MCCos(val float64) float64 {
	return mcSinTable[uint16(val*10430.378+16384.0)&65535]
}

// ClampFloat clamps the given value to the given range.
func ClampFloat(num, min, max float64) float64 {
	if num < min {
		return min
	}
	return math.Min(num, max)
}

// Vec3HzDistSqr returns the squared horizontal distance in a vector.
func Vec3HzDistSqr(vec3 mgl64.Vec3) float64 {
	return vec3.X()*vec3.X() + vec3.Z()*vec3.Z()
}
