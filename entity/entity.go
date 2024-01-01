package entity

import "github.com/go-gl/mathgl/mgl32"

const (
	EntityPlayerInterpolationTicks = 3
	EntityMobInterpolationTicks    = 6
)

type Entity struct {
	Position, PreviousPosition mgl32.Vec3
	RecvPosition               mgl32.Vec3
	PositionHistory            []HistoricalPosition

	HistorySize        int
	InterpolationTicks int

	IsPlayer bool
}

// New creates and returns a new Entity instance.
func New(pos, vel mgl32.Vec3, historySize int, isPlayer bool) *Entity {
	e := &Entity{
		Position:         pos,
		PreviousPosition: pos,
		RecvPosition:     pos.Add(vel),

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
	e.PreviousPosition = e.Position
	e.Position = hp.Position

	e.PositionHistory = append(e.PositionHistory, HistoricalPosition{
		Position: e.PreviousPosition,
		Tick:     hp.Tick,
	})

	if len(e.PositionHistory) > e.HistorySize {
		e.PositionHistory = e.PositionHistory[1:]
	}
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
