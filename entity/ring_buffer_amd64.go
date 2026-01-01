//go:build amd64
// +build amd64

package entity

import "github.com/go-gl/mathgl/mgl32"

// distanceSqr calculates the squared distance between two Vec3 positions using SIMD
// This function is implemented in ring_buffer_amd64.s
func distanceSqr(a, b mgl32.Vec3) float32
