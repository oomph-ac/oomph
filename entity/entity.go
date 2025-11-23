package entity

import (
	"log/slog"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/utils"
)

const (
	EntityPlayerInterpolationTicks = 3
	EntityMobInterpolationTicks    = 6
)

type Entity struct {
	RuntimeId uint64

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

	PositionHistory *utils.CircularQueue[HistoricalPosition]

	InterpolationTicks int
	TicksSinceTeleport int

	Width  float32
	Height float32
	Scale  float32

	IsPlayer bool

	historySize int
	log         **slog.Logger
}

// New creates and returns a new Entity instance.
func New(
	runtimeId uint64,
	entType string,
	metadata map[uint32]any,
	pos mgl32.Vec3,
	historySize int,
	isPlayer bool,
	width, height, scale float32,
	log **slog.Logger,
) *Entity {
	e := &Entity{
		RuntimeId: runtimeId,

		Type:     entType,
		Metadata: metadata,

		Position:     pos,
		PrevPosition: pos,
		RecvPosition: pos,

		Width:  width,
		Height: height,
		Scale:  scale,

		PositionHistory: utils.NewCircularQueue(historySize, func() (hp HistoricalPosition) { return }),

		IsPlayer: isPlayer,

		log:         log,
		historySize: historySize,
	}
	/* e.InterpolationTicks = EntityMobInterpolationTicks
	if isPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	} */

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
func (e *Entity) UpdatePosition(hp HistoricalPosition) error {
	e.PrevPosition = e.Position
	e.Position = hp.Position

	e.PrevVelocity = e.Velocity
	e.Velocity = e.Position.Sub(e.PrevPosition)
	if err := e.PositionHistory.Append(hp); err != nil {
		return err
	}
	return nil
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
func (e *Entity) Tick(tick int64) error {
	if e.InterpolationTicks < 0 {
		return e.UpdatePosition(HistoricalPosition{
			Position:     e.Position,
			PrevPosition: e.Position,
			Tick:         tick,
		})
	}

	newPos := e.Position
	if e.InterpolationTicks > 0 {
		delta := e.RecvPosition.Sub(e.Position).Mul(1 / float32(e.InterpolationTicks))
		newPos = newPos.Add(delta)
	} else {
		newPos = e.RecvPosition
	}

	if err := e.UpdatePosition(HistoricalPosition{
		Position:     newPos,
		PrevPosition: e.Position,
		Tick:         tick,
	}); err != nil {
		return err
	}
	e.TicksSinceTeleport++
	e.InterpolationTicks--
	return nil
}

func (e *Entity) debug(msg string, args ...any) {
	if log := *e.log; log != nil {
		log.Debug(msg, args...)
	}
}
