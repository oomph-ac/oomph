package entity

import (
	"github.com/go-gl/mathgl/mgl64"
)

// Location represents a location of an entity.
type Location struct {
	// Position is the current position of the entity in the world.
	Position mgl64.Vec3
	// LastPosition is the previous position of the entity in the world.
	LastPosition mgl64.Vec3
	// RecievedPosition is the position of the entity that the client sees on their side
	RecievedPosition mgl64.Vec3
	// Rotation represents the rotation of an entity. The first value is the pitch, and the second and third represent
	// the yaw and head yaw, the latter of which only being applicable for certain entities, such as players.
	Rotation mgl64.Vec3
	// LastRotation is the rotation that the entity was in right before Rotation was updated.
	LastRotation mgl64.Vec3
	// TeleportTicks is the amount of client ticks that have passed since the entity has teleported.
	TeleportTicks uint32
	// NewPosRotationIncrements is the amount of ticks the entity's position should be smoothed out by.
	// Every client tick, this value will be de-incremented, and whenever the client recieves a position for an
	// entity, this value will be reset to 3.
	NewPosRotationIncrements uint32
}
