package entity

import (
	"log/slog"
	"maps"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/anticheat/utils"
)

const (
	EntityPlayerInterpolationTicks = 3
	EntityMobInterpolationTicks    = 6
)

type Entity struct {
	RuntimeId uint64
	UniqueId  int64

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

	NetworkOffset float32

	IsPlayer bool

	historySize int
	log         **slog.Logger
}

// Config ...
type Config struct {
	RuntimeID uint64
	UniqueID  int64

	Type     string
	Metadata map[uint32]any

	NetworkPosition mgl32.Vec3
	HistorySize     int
	IsPlayer        bool

	Width, Height, Scale float32

	Log **slog.Logger
}

// New creates and returns a new Entity instance.
func New(c Config) *Entity {
	metadata := maps.Clone(c.Metadata)
	if metadata == nil {
		metadata = make(map[uint32]any)
	}
	offset := networkOffset(c.Type, metadata)
	pos := c.NetworkPosition
	pos[1] -= offset

	e := &Entity{
		RuntimeId: c.RuntimeID,
		UniqueId:  c.UniqueID,

		Type:     c.Type,
		Metadata: metadata,

		Position:     pos,
		PrevPosition: pos,
		RecvPosition: pos,

		Width:  c.Width,
		Height: c.Height,
		Scale:  c.Scale,

		NetworkOffset: offset,

		PositionHistory: utils.NewCircularQueue(c.HistorySize, func() (hp HistoricalPosition) { return }),

		IsPlayer: c.IsPlayer,

		log:         c.Log,
		historySize: c.HistorySize,
	}
	/* e.InterpolationTicks = EntityMobInterpolationTicks
	if isPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	} */

	return e
}

// ReceivePosition updates the position of the entity, and adds the previous position to its position history.
func (e *Entity) ReceivePosition(pos mgl32.Vec3, teleport bool) {
	e.PrevRecvPosition = e.RecvPosition
	e.RecvPosition = pos

	e.InterpolationTicks = EntityMobInterpolationTicks
	if e.IsPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	}

	if teleport {
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

// UpdateMetadata ...
func (e *Entity) UpdateMetadata(metadata map[uint32]any) {
	if e.Metadata == nil {
		e.Metadata = make(map[uint32]any, len(metadata))
	}
	maps.Copy(e.Metadata, metadata)
	e.NetworkOffset = networkOffset(e.Type, e.Metadata)
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
