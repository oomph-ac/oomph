//go:build !amd64 && !arm64
// +build !amd64,!arm64

package entity

import "github.com/go-gl/mathgl/mgl32"

// distanceSqr calculates the squared distance between two Vec3 positions (fallback implementation)
func distanceSqr(a, b mgl32.Vec3) float32 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	dz := a[2] - b[2]
	return dx*dx + dy*dy + dz*dz
}
