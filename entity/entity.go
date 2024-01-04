package entity

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	EntityPlayerInterpolationTicks = 4
	EntityMobInterpolationTicks    = 6
)

type Entity struct {
	Position, PrevPosition mgl32.Vec3
	Velocity, PrevVelocity mgl32.Vec3
	RecvPosition           mgl32.Vec3

	HistorySize     int
	PositionHistory []HistoricalPosition

	InterpolationTicks int

	Width  float32
	Height float32

	IsPlayer bool
}

// New creates and returns a new Entity instance.
func New(pos, vel mgl32.Vec3, historySize int, isPlayer bool) *Entity {
	e := &Entity{
		Position:     pos,
		PrevPosition: pos,
		RecvPosition: pos.Add(vel),

		Width:  0.6,
		Height: 1.8,

		HistorySize: historySize,
		IsPlayer:    isPlayer,
	}

	e.InterpolationTicks = EntityMobInterpolationTicks
	if isPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	}

	return e
}

// RecievePosition updates the position of the entity, and adds the previous position to the position history.
func (e *Entity) RecievePosition(hp HistoricalPosition) {
	e.RecvPosition = hp.Position
	e.InterpolationTicks = EntityMobInterpolationTicks
	if e.IsPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	}
}

// UpdatePosition updates the position of the entity, and adds the previous position to the position history.
func (e *Entity) UpdatePosition(hp HistoricalPosition) {
	e.PrevPosition = e.Position
	e.Position = hp.Position

	e.PrevVelocity = e.Velocity
	e.Velocity = e.Position.Sub(e.PrevPosition)

	e.PositionHistory = append(e.PositionHistory, hp)
	if len(e.PositionHistory) > e.HistorySize {
		e.PositionHistory = e.PositionHistory[1:]
	}
}

// Box returns the entity's bounding box.
func (e *Entity) Box(pos mgl32.Vec3) cube.BBox {
	return cube.Box(-e.Width/2, 0, -e.Width/2, e.Width/2, e.Height, e.Width/2).Translate(pos)
}

// Tick updates the entity's position based on the interpolation ticks.
func (e *Entity) Tick(tick int64) {
	pos := e.Position
	if e.InterpolationTicks > 0 {
		delta := e.RecvPosition.Sub(e.Position).Mul(1 / float32(e.InterpolationTicks))
		pos = pos.Add(delta)
		e.InterpolationTicks--
	}

	e.UpdatePosition(HistoricalPosition{
		Position: pos,
		Tick:     tick,
	})
}
