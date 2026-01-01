//go:build arm64
// +build arm64

package entity

import "github.com/go-gl/mathgl/mgl32"

// distanceSqr calculates the squared distance between two Vec3 positions using SIMD
// This function is implemented in ring_buffer_arm64.s
func distanceSqr(a, b mgl32.Vec3) float32
