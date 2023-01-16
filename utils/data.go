package utils

import "github.com/go-gl/mathgl/mgl32"

type AuthorityType byte
type CollisionType byte

const (
	ModeClientAuthoritative AuthorityType = iota
	ModeSemiAuthoritative
	ModeFullAuthoritative
)

const (
	CollisionX CollisionType = iota
	CollisionY
	CollisionZ
)

type LocationData struct {
	Tick     uint64
	Position mgl32.Vec3
	OnGround bool
	Teleport bool
}
