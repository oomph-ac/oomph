package utils

import "github.com/go-gl/mathgl/mgl32"

type AuthorityType byte

const (
	ModeClientAuthoritative AuthorityType = iota
	ModeSemiAuthoritative
	ModeFullAuthoritative
)

type LocationData struct {
	Tick     uint64
	Position mgl32.Vec3
	OnGround bool
	Teleport bool
}
