package entity

import (
	"sync"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/utils"
)

// Entity represents an entity in a world with a *player.Player.
type Entity struct {
	// mu protects all the following fields.
	mu sync.Mutex
	// position is the current position of the entity in the world.
	position mgl32.Vec3
	// lastPosition is the previous position of the entity in the world.
	lastPosition mgl32.Vec3
	// recievedPosition is the position of the entity on the server-side
	recievedPosition mgl32.Vec3
	// serverPosition is the position of the entity on the server-side
	serverPosition mgl32.Vec3
	// newPosRotationIncrements is used for smoothing out the position of entities.
	newPosRotationIncrements int
	// positionBuffer is the buffer of the previous positions of the entity.
	positionBuffer []utils.LocationData
	// updatedTick is the last tick the entity's position was updated
	updatedTick uint64
	// rotation represents the rotation of an entity. The first value is the pitch, and the second and third represent
	// the yaw and head yaw, the latter of which only being applicable for certain entities, such as players.
	rotation mgl32.Vec3
	// lastRotation is the rotation that the entity was in right before rotation was updated.
	lastRotation mgl32.Vec3
	// teleportTicks is the amount of client ticks that have passed since the entity has teleported.
	teleportTicks uint32
	// aabb represents the bounding box of the entity.
	aabb cube.BBox
	// player is true if the entity is a player.
	player bool
	// onGround determines wether the entity is on or off the ground
	onGround bool
}

// defaultAABB is the default AABB for newly created entities.
var defaultAABB = cube.Box(
	-0.3, 0, -0.3,
	0.3, 1.8, 0.3,
)

// NewEntity creates a new entity with the provided parameters.
func NewEntity(position, velocity, rotation mgl32.Vec3, player bool) *Entity {
	return &Entity{
		position:                 position,
		lastPosition:             position,
		recievedPosition:         position.Add(velocity),
		rotation:                 rotation,
		lastRotation:             rotation,
		aabb:                     defaultAABB,
		player:                   player,
		onGround:                 true,
		newPosRotationIncrements: 3,
	}
}

// SetServerPosition sets the position of the entity on the server-side.
func (e *Entity) SetServerPosition(pos mgl32.Vec3) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.player {
		pos[1] -= 1.62
	}

	e.serverPosition = pos
}

// ServerPosition returns the position of the entity on the server-side.
func (e *Entity) ServerPosition() mgl32.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.serverPosition
}

// RecievedPosition returns the position of the entity that the client has recieved.
func (e *Entity) RecievedPosition() mgl32.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.recievedPosition
}

// Position returns the position of the entity.
func (e *Entity) Position() mgl32.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.position
}

// LastPosition returns the last position of the entity.
func (e *Entity) LastPosition() mgl32.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastPosition
}

// Move moves the entity to the provided position.
func (e *Entity) Move(pos mgl32.Vec3, offset bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.lastPosition = e.position
	e.position = pos
	if offset {
		e.position[1] -= 1.62
	}
}

func (e *Entity) TickPosition(tick uint64) {
	if e.newPosRotationIncrements > 0 {
		delta := e.recievedPosition.Sub(e.position)
		e.lastPosition = e.position
		e.position = e.position.Add(delta.Mul(1 / float32(e.newPosRotationIncrements)))
		e.newPosRotationIncrements--
	}

	e.positionBuffer = append(e.positionBuffer, utils.LocationData{Tick: tick, Position: e.position})
	if len(e.positionBuffer) > 6 { // 6 = player.NetworkLatenctyCutoff
		e.positionBuffer = e.positionBuffer[1:]
	}
}

// UpdatePosition updates the position of the entity that the client sees.
func (e *Entity) UpdatePosition(dat utils.LocationData, offset bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.recievedPosition = dat.Position
	e.onGround = dat.OnGround

	if dat.Teleport {
		e.teleportTicks = 0
		e.newPosRotationIncrements = 1
	} else {
		e.teleportTicks++
		e.newPosRotationIncrements = 3
	}

	if offset {
		e.recievedPosition[1] -= 1.62
	}

	e.updatedTick = dat.Tick
}

func (e *Entity) HasUpdated(tick uint64) bool {
	return e.updatedTick == tick
}

// RewindPosition rewinds the position of the entity to the provided tick. This is mainly
// used for server authoritative combat.
func (e *Entity) RewindPosition(tick uint64) *utils.LocationData {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, dat := range e.positionBuffer {
		if dat.Tick == tick {
			return &dat
		}
	}

	return nil
}

// PositionBuffer returns the position buffer of the entity.
func (e *Entity) PositionBuffer() []utils.LocationData {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.positionBuffer
}

// Rotation returns the rotation of the entity.
func (e *Entity) Rotation() mgl32.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.rotation
}

// LastRotation returns the last rotation of the entity.
func (e *Entity) LastRotation() mgl32.Vec3 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastRotation
}

// Rotate rotates the entity to the provided rotation.
func (e *Entity) Rotate(rotation mgl32.Vec3) {
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
func (e *Entity) AABB() cube.BBox {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.aabb
}

// SetAABB updates the AABB of the entity.
func (e *Entity) SetAABB(aabb cube.BBox) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.aabb = aabb
}
