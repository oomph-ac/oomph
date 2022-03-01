package game

import "math"

// sinTable ...
var sinTable []float64

// init initializes the sinTable.
func init() {
	for i := 0.0; i < 65536; i++ {
		sinTable = append(sinTable, math.Sin(i*math.Pi*2/65536))
	}
}

// MCSin returns the Minecraft sin of the given angle.
func MCSin(val float64) float64 {
	return sinTable[uint16(val*10430.378)&65535]
}

// MCCos returns the Minecraft cos of the given angle.
func MCCos(val float64) float64 {
	return sinTable[uint16(val*10430.378+16384.0)&65535]
}

// ClampFloat clamp the given value to the given range.
func ClampFloat(num, min, max float64) float64 {
	if num < min {
		return min
	}
	return math.Min(num, max)
}
