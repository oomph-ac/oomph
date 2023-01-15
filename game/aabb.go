package game

import (
	"github.com/chewxy/math32"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// AABBFromDFBox converts a dragonfly bounding box to a float32-cube bounding box.
func DFBoxToCubeBox(b df_cube.BBox) cube.BBox {
	return cube.Box(
		float32(b.Min().X()), float32(b.Min().Y()), float32(b.Min().Z()),
		float32(b.Max().X()), float32(b.Max().Y()), float32(b.Max().Z()),
	)
}

// CubeBoxToDFBox converts a float32-cube bounding box to a dragonfly bounding box.
func CubeBoxToDFBox(b cube.BBox) df_cube.BBox {
	return df_cube.Box(
		float64(b.Min().X()), float64(b.Min().Y()), float64(b.Min().Z()),
		float64(b.Max().X()), float64(b.Max().Y()), float64(b.Max().Z()),
	)
}

// AABBFromDimensions returns a bounding box from the given dimensions.
func AABBFromDimensions(width, height float32) cube.BBox {
	h := width / 2
	return cube.Box(
		-h, 0, -h,
		h, height, h,
	)
}

// AABBVectorDistance calculates the distance between an AABB and a vector.
func AABBVectorDistance(a cube.BBox, v mgl32.Vec3) float32 {
	x := math32.Max(a.Min().X()-v.X(), math32.Max(0, v.X()-a.Max().X()))
	y := math32.Max(a.Min().Y()-v.Y(), math32.Max(0, v.Y()-a.Max().Y()))
	z := math32.Max(a.Min().Z()-v.Z(), math32.Max(0, v.Z()-a.Max().Z()))

	dist := math32.Sqrt(math32.Pow(x, 2) + math32.Pow(y, 2) + math32.Pow(z, 2))
	if dist == mgl32.NaN {
		dist = 0
	}

	return dist
}
