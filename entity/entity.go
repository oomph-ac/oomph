package entity

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	EntityPlayerInterpolationTicks = 3
	EntityMobInterpolationTicks    = 6
)

type Entity struct {
	Metadata map[uint32]any
	Type     string

	// Position is the current position of the entity, after interpolation.
	Position, PrevPosition mgl32.Vec3
	// RecvPosition is the position of the entity received by the client. It
	// is used as the end point for interpolation.
	RecvPosition, PrevRecvPosition mgl32.Vec3

	// Velocity is the current position of the entity subtracted by the
	Velocity, PrevVelocity mgl32.Vec3
	// RecvVelocity is the velocity of the entity sent by the server in SetActorMotion.
	RecvVelocity, PrevRecvVelocity mgl32.Vec3

	PositionHistory []HistoricalPosition

	InterpolationTicks int
	TicksSinceTeleport int

	Width  float32
	Height float32
	Scale  float32

	IsPlayer bool
}

// New creates and returns a new Entity instance.
func New(entType string, metadata map[uint32]any, pos, vel mgl32.Vec3, historySize int, isPlayer bool, width, height, scale float32) *Entity {
	e := &Entity{
		Type:     entType,
		Metadata: metadata,

		Position:     pos,
		PrevPosition: pos,
		RecvPosition: pos.Add(vel),

		Width:  width,
		Height: height,
		Scale:  scale,

		PositionHistory: make([]HistoricalPosition, 0, historySize),

		IsPlayer: isPlayer,
	}

	e.InterpolationTicks = EntityMobInterpolationTicks
	if isPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	}

	return e
}

// ReceivePosition updates the position of the entity, and adds the previous position to its position history.
func (e *Entity) ReceivePosition(hp HistoricalPosition) {
	e.PrevRecvPosition = e.RecvPosition
	e.RecvPosition = hp.Position

	e.InterpolationTicks = EntityMobInterpolationTicks
	if e.IsPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	}

	if hp.Teleport {
		e.TicksSinceTeleport = 0
		e.InterpolationTicks = 1
	}
}

// UpdatePosition updates the position of the entity, and adds the previous position to the position history.
func (e *Entity) UpdatePosition(hp HistoricalPosition) {
	e.PrevPosition = e.Position
	e.Position = hp.Position

	e.PrevVelocity = e.Velocity
	e.Velocity = e.Position.Sub(e.PrevPosition)

	if cap(e.PositionHistory) == len(e.PositionHistory) {
		e.PositionHistory = e.PositionHistory[1:]
	}
	e.PositionHistory = append(e.PositionHistory, hp)
}

// UpdateVelocity updates the velocity of the entity.
func (e *Entity) UpdateVelocity(vel mgl32.Vec3) {
	e.PrevRecvVelocity = e.RecvVelocity
	e.RecvVelocity = vel
}

// Box returns the entity's bounding box.
func (e *Entity) Box(pos mgl32.Vec3) cube.BBox {
	w := (e.Width * e.Scale) / 2
	return cube.Box(
		-w,
		0,
		-w,
		w,
		e.Height*e.Scale,
		w,
	).Translate(pos)
}

// BoxExpansion returns the amount the bounding box of the entity should be extended
// by to calculate combat reach.
func (e *Entity) BoxExpansion() float32 {
	return 0.1 * e.Scale
}

// Tick updates the entity's position based on the interpolation ticks.
func (e *Entity) Tick(tick int64) {
	if e.InterpolationTicks < 0 {
		return
	}

	newPos := e.Position
	if e.InterpolationTicks > 0 {
		delta := e.RecvPosition.Sub(e.Position).Mul(1 / float32(e.InterpolationTicks))
		newPos = newPos.Add(delta)
	} else {
		newPos = e.RecvPosition
	}

	e.UpdatePosition(HistoricalPosition{
		Position:     newPos,
		PrevPosition: e.Position,
		Tick:         tick,
	})
	e.TicksSinceTeleport++
	e.InterpolationTicks--
}
