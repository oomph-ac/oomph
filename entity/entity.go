package entity

import (
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/go-gl/mathgl/mgl64"
	"sync"
)

// Entity represents an entity in a world with a *player.Player.
type Entity struct {
	// mu protects all the following fields.
	mu sync.Mutex
	// position is the current position of the entity in the world.
	position mgl64.Vec3
	// lastPosition is the previous position of the entity in the world.
	lastPosition mgl64.Vec3
	// receivedPosition is the position of the entity that the client sees on their side.
	receivedPosition mgl64.Vec3
	// rotation represents the rotation of an entity. The first value is the pitch, and the second and third represent
	// the yaw and head yaw, the latter of which only being applicable for certain entities, such as players.
	rotation mgl64.Vec3
	// lastRotation is the rotation that the entity was in right before rotation was updated.
	lastRotation mgl64.Vec3
	// teleportTicks is the amount of client ticks that have passed since the entity has teleported.
	teleportTicks uint32
	// aabb represents the bounding box of the entity.
	aabb physics.AABB
	// player is true if the entity is a player.
	player bool
	// newPosRotationIncrements is the amount of ticks the entity's position should be smoothed out by.
	// Every client tick, this value will be de-incremented, and whenever the client receives a position for an
	// entity, this value will be reset to 3.
	newPosRotationIncrements uint32
}

// defaultAABB is the default AABB for newly created entities.
var defaultAABB = physics.NewAABB(
	mgl64.Vec3{-0.3, 0, -0.3},
	mgl64.Vec3{0.3, 1.8, 0.3},
)

// NewEntity creates a new entity with the provided parameters.
func NewEntity(position, velocity, rotation mgl64.Vec3, player bool) *Entity {
	return &Entity{
		position:                 position,
		lastPosition:             position,
		receivedPosition:         position.Add(velocity),
		rotation:                 rotation,
		lastRotation:             rotation,
		aabb:                     defaultAABB,
		player:                   player,
		newPosRotationIncrements: 3,
	}
}

// Position returns the position of the entity.
func (e *Entity) Position() mgl64.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.position
}

// LastPosition returns the last position of the entity.
func (e *Entity) LastPosition() mgl64.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastPosition
}

// Move moves the entity to the provided position.
func (e *Entity) Move(pos mgl64.Vec3) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastPosition = e.position
	e.position = pos.Sub(mgl64.Vec3{0, 1.62})
}

// ReceivedPosition returns the position of the entity that the client sees.
func (e *Entity) ReceivedPosition() mgl64.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.receivedPosition
}

// UpdateReceivedPosition updates the position of the entity that the client sees.
func (e *Entity) UpdateReceivedPosition(position mgl64.Vec3) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.receivedPosition = position
}

// Rotation returns the rotation of the entity.
func (e *Entity) Rotation() mgl64.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.rotation
}

// LastRotation returns the last rotation of the entity.
func (e *Entity) LastRotation() mgl64.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastRotation
}

// Rotate rotates the entity to the provided rotation.
func (e *Entity) Rotate(rotation mgl64.Vec3) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastRotation = e.rotation
	e.rotation = rotation
}

// TeleportationTicks returns the amount of ticks that have passed since the entity has teleported.
func (e *Entity) TeleportationTicks() uint32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.teleportTicks
}

// ResetTeleportationTicks resets the teleportation ticks to zero.
func (e *Entity) ResetTeleportationTicks() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.teleportTicks = 0
}

// IncrementTeleportationTicks increments the teleportation ticks of the entity.
func (e *Entity) IncrementTeleportationTicks() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.teleportTicks++
}

// NewPositionRotationIncrements returns the amount of ticks the entity's position should be smoothed out by.
func (e *Entity) NewPositionRotationIncrements() uint32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.newPosRotationIncrements
}

// DecrementNewPositionRotationIncrements decrements the amount of ticks the entity's position should be smoothed
// out by.
func (e *Entity) DecrementNewPositionRotationIncrements() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.newPosRotationIncrements--
}

// ResetNewPositionRotationIncrements resets the amount of ticks the entity's position should be smoothed out by to
// three.
func (e *Entity) ResetNewPositionRotationIncrements() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.newPosRotationIncrements = 3
}

// Player returns true if the entity is a player.
func (e *Entity) Player() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.player
}

// AABB returns the AABB of the entity.
func (e *Entity) AABB() physics.AABB {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.aabb
}

// SetAABB updates the AABB of the entity.
func (e *Entity) SetAABB(aabb physics.AABB) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.aabb = aabb
}
