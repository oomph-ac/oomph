package game

import (
	"fmt"
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// AABBFromDimensions returns a bounding box from the given dimensions.
func AABBFromDimensions(width, height float64) cube.BBox {
	h := width / 2
	return cube.Box(
		-h, 0, -h,
		h, height, h,
	)
}

// AABBVectorDistance calculates the distance between an AABB and a vector.
func AABBVectorDistance(a cube.BBox, v mgl64.Vec3) float64 {
	x := math.Max(a.Min().X()-v.X(), math.Max(0, v.X()-a.Max().X()))
	y := math.Max(a.Min().Y()-v.Y(), math.Max(0, v.Y()-a.Max().Y()))
	z := math.Max(a.Min().Z()-v.Z(), math.Max(0, v.Z()-a.Max().Z()))
	return math.Sqrt(math.Pow(x, 2) + math.Pow(y, 2) + math.Pow(z, 2))
}

// XOffset calculates the offset on the X axis between two bounding boxes, returning a delta always
// smaller than or equal to deltaX if deltaX is bigger than 0, or always bigger than or equal to deltaX if it
// is smaller than 0.
// This is copied from Dragonfly and modified to handle the precision error causing a faulty check in the bounding box.
func XOffset(origin cube.BBox, nearby cube.BBox, deltaX float64) float64 {
	if origin.Max()[1] <= nearby.Min()[1] || origin.Min()[1] >= nearby.Max()[1] {
		return deltaX
	} else if origin.Max()[2] <= nearby.Min()[2] || origin.Min()[2] >= nearby.Max()[2] {
		return deltaX
	}
	if deltaX > 0 && Round(origin.Max()[0], 3) <= nearby.Min()[0] {
		difference := nearby.Min()[0] - Round(origin.Max()[0], 4)
		if difference < deltaX {
			deltaX = difference
		}
	} else {
		fmt.Println(origin.Max()[0], nearby.Min()[0])
	}
	if deltaX < 0 && Round(origin.Min()[0], 3) >= nearby.Max()[0] {
		difference := nearby.Max()[0] - Round(origin.Min()[0], 4)

		if difference > deltaX {
			deltaX = difference
		}
	}
	return deltaX
}

// ZOffset calculates the offset on the Z axis between two bounding boxes, returning a delta always
// smaller than or equal to deltaZ if deltaZ is bigger than 0, or always bigger than or equal to deltaZ if it
// is smaller than 0.
// This is copied from Dragonfly and modified to handle the precision error causing a faulty check in the bounding box.
func ZOffset(origin cube.BBox, nearby cube.BBox, deltaZ float64) float64 {
	if origin.Max()[0] <= nearby.Min()[0] || origin.Min()[0] >= nearby.Max()[0] {
		return deltaZ
	} else if origin.Max()[1] <= nearby.Min()[1] || origin.Min()[1] >= nearby.Max()[1] {
		return deltaZ
	}
	if deltaZ > 0 && Round(origin.Max()[2], 3) <= nearby.Min()[2] {
		difference := nearby.Min()[2] - Round(origin.Max()[2], 4)
		if difference < deltaZ {
			deltaZ = difference
		}
	}
	if deltaZ < 0 && Round(origin.Min()[2], 3) >= nearby.Max()[2] {
		difference := nearby.Max()[2] - Round(origin.Min()[2], 4)

		if difference > deltaZ {
			deltaZ = difference
		}
	}
	return deltaZ
}
