package game

import "github.com/chewxy/math32"

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
