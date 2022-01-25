package util

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// Vec32To64 converts a mgl32.Vec3 to a mgl64.Vec3.
func Vec32To64(vec3 mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(vec3[0]), float64(vec3[1]), float64(vec3[2])}
}

// CubePosFromProtocolBlockPos converts a protocol.BlockPos to a cube.Pos.
func CubePosFromProtocolBlockPos(pos protocol.BlockPos) cube.Pos {
	return cube.Pos{int(pos.X()), int(pos.Y()), int(pos.Z())}
}
