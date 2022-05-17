package entity

import (
	"sync"

	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/go-gl/mathgl/mgl64"
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
	// interpolationMotion is the client-sided motion of the entity
	interpolatedMotion mgl64.Vec3
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
	// onGround determines wether the entity is on or off the ground
	onGround bool
	// newLocationIncrements is the amount of ticks the entity's position should be smoothed out by.
	// Every client tick, this value will be de-incremented, and whenever the client receives a position for an
	// entity, this value will be reset to 3.
	newLocationIncrements uint32
}

// defaultAABB is the default AABB for newly created entities.
var defaultAABB = physics.NewAABB(
	mgl64.Vec3{-0.3, 0, -0.3},
	mgl64.Vec3{0.3, 1.8, 0.3},
)

// NewEntity creates a new entity with the provided parameters.
func NewEntity(position, velocity, rotation mgl64.Vec3, player bool) *Entity {
	return &Entity{
		position:         position,
		lastPosition:     position,
		receivedPosition: position.Add(velocity),
		rotation:         rotation,
		lastRotation:     rotation,
		aabb:             defaultAABB,
		player:           player,
		onGround:         true,
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
func (e *Entity) Move(pos mgl64.Vec3, offset bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastPosition = e.position
	e.position = pos
	if offset {
		e.position[1] -= 1.62
	}
}

func (e *Entity) ClientInterpolation(motion mgl64.Vec3) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.interpolatedMotion = motion
}

func (e *Entity) InterpolatedMotion() mgl64.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.interpolatedMotion
}

// ReceivedPosition returns the position of the entity that the client sees.
func (e *Entity) ReceivedPosition() mgl64.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.receivedPosition
}

// UpdateReceivedPosition updates the position of the entity that the client sees.
func (e *Entity) UpdateReceivedPosition(pos mgl64.Vec3, ground bool, offset bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.receivedPosition = pos
	e.onGround = ground
	if offset {
		e.receivedPosition[1] -= 1.62
	}
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

// NewLocationIncrements returns the amount of ticks the entity's position should be smoothed out by.
func (e *Entity) NewLocationIncrements() uint32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.newLocationIncrements
}

// DecrementNewLocationIncrements decrements the amount of ticks the entity's position should be smoothed
// out by.
func (e *Entity) DecrementNewLocationIncrements() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.newLocationIncrements--
}

// ResetNewLocationIncrements resets the amount of ticks the entity's position should be smoothed out by to
// three.
func (e *Entity) ResetNewLocationIncrements() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.newLocationIncrements = 3
}

// Player returns true if the entity is a player.
func (e *Entity) Player() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.player
}

func (e *Entity) OnGround() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.onGround
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
