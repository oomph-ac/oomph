package utils

import "github.com/go-gl/mathgl/mgl64"

type LocationData struct {
	Tick     uint64
	Position mgl64.Vec3
	OnGround bool
}
