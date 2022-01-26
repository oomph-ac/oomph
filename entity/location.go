package entity

import "github.com/go-gl/mathgl/mgl64"

// Location represents a location of an entity.
type Location struct {
	// Position is the position of the entity in the world.
	Position mgl64.Vec3
	// LastPosition is the position that the entity was in right before Position was updated.
	LastPosition mgl64.Vec3
	// Rotation represents the rotation of an entity. The first value is the pitch, and the second and third represent
	// the yaw and head yaw, the latter of which only being applicable for certain entities, such as players.
	Rotation mgl64.Vec3
	// LastRotation is the rotation that the entity was in right before Rotation was updated.
	LastRotation mgl64.Vec3
}
