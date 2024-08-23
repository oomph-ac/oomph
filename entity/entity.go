package entity

import (
	"bytes"
	"encoding/json"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/utils"
)

const (
	EntityPlayerInterpolationTicks = 3
	EntityMobInterpolationTicks    = 6
)

type Entity struct {
	// Position is the current position of the entity, after interpolation.
	Position, PrevPosition mgl32.Vec3
	// RecvPosition is the position of the entity recieved by the client. It
	// is used as the end point for interpolation.
	RecvPosition, PrevRecvPosition mgl32.Vec3

	// Velocity is the current position of the entity subtracted by the
	Velocity, PrevVelocity mgl32.Vec3
	// RecvVelocity is the velocity of the entity sent by the server in SetActorMotion.
	RecvVelocity, PrevRecvVelocity mgl32.Vec3

	HistorySize     int
	PositionHistory []HistoricalPosition

	InterpolationTicks int
	TicksSinceTeleport int

	Width  float32
	Height float32
	Scale  float32

	IsPlayer bool
}

// New creates and returns a new Entity instance.
func New(pos, vel mgl32.Vec3, historySize int, isPlayer bool, width, height, scale float32) *Entity {
	e := &Entity{
		Position:     pos,
		PrevPosition: pos,
		RecvPosition: pos.Add(vel),

		Width:  width,
		Height: height,
		Scale:  scale,

		HistorySize: historySize,
		IsPlayer:    isPlayer,
	}

	e.InterpolationTicks = EntityMobInterpolationTicks
	if isPlayer {
		e.InterpolationTicks = EntityPlayerInterpolationTicks
	}

	return e
}

func (e *Entity) Encode(buf *bytes.Buffer) {
	utils.WriteVec32(buf, e.Position)
	utils.WriteVec32(buf, e.PrevPosition)
	utils.WriteVec32(buf, e.RecvPosition)
	utils.WriteVec32(buf, e.PrevRecvPosition)

	utils.WriteVec32(buf, e.Velocity)
	utils.WriteVec32(buf, e.PrevVelocity)
	utils.WriteVec32(buf, e.RecvVelocity)
	utils.WriteVec32(buf, e.PrevRecvVelocity)

	utils.WriteLInt32(buf, int32(e.HistorySize))
	enc, err := json.Marshal(e.PositionHistory)
	if err != nil {
		panic(err)
	}

	utils.WriteLInt32(buf, int32(len(enc)))
	buf.Write(enc)

	utils.WriteLInt32(buf, int32(e.InterpolationTicks))
	utils.WriteLInt32(buf, int32(e.TicksSinceTeleport))

	utils.WriteLFloat32(buf, e.Width)
	utils.WriteLFloat32(buf, e.Height)

	utils.WriteBool(buf, e.IsPlayer)
}

func Decode(buf *bytes.Buffer) *Entity {
	e := &Entity{}
	e.Position = utils.ReadVec32(buf.Next(12))
	e.PrevPosition = utils.ReadVec32(buf.Next(12))
	e.RecvPosition = utils.ReadVec32(buf.Next(12))
	e.PrevRecvPosition = utils.ReadVec32(buf.Next(12))

	e.Velocity = utils.ReadVec32(buf.Next(12))
	e.PrevVelocity = utils.ReadVec32(buf.Next(12))
	e.RecvVelocity = utils.ReadVec32(buf.Next(12))
	e.PrevRecvVelocity = utils.ReadVec32(buf.Next(12))

	e.HistorySize = int(utils.LInt32(buf.Next(4)))

	len := int(utils.LInt32(buf.Next(4)))
	err := json.Unmarshal(buf.Next(len), &e.PositionHistory)
	if err != nil {
		panic(err)
	}

	e.InterpolationTicks = int(utils.LInt32(buf.Next(4)))
	e.TicksSinceTeleport = int(utils.LInt32(buf.Next(4)))

	e.Width = utils.LFloat32(buf.Next(4))
	e.Height = utils.LFloat32(buf.Next(4))

	e.IsPlayer = utils.Bool(buf.Next(1))
	return e
}

// RecievePosition updates the position of the entity, and adds the previous position to its position history.
func (e *Entity) RecievePosition(hp HistoricalPosition) {
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

	e.PositionHistory = append(e.PositionHistory, hp)
	if len(e.PositionHistory) > e.HistorySize {
		e.PositionHistory = e.PositionHistory[1:]
	}
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
	e.TicksSinceTeleport++
}
