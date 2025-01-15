package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	DEFAULT_MAX_REWIND_TICKS int = 6
)

// EntityTrackerComponent is a component that handles entities that the member player is
// viewing on their screen.
type EntityTrackerComponent struct {
	mPlayer *player.Player

	entities       map[uint64]*entity.Entity
	maxRewindTicks int
	ackDependent   bool
}

func NewEntityTrackerComponent(p *player.Player, ackDependent bool) *EntityTrackerComponent {
	return &EntityTrackerComponent{
		mPlayer: p,

		entities:       make(map[uint64]*entity.Entity),
		maxRewindTicks: DEFAULT_MAX_REWIND_TICKS,
		ackDependent:   ackDependent,
	}
}

// AddEntity adds an entity to the entity tracker component.
func (c *EntityTrackerComponent) AddEntity(rid uint64, ent *entity.Entity) {
	c.entities[rid] = ent
	if !c.ackDependent || ent.IsPlayer {
		return
	}

	// Check if the current entity is a firework and will in turn, end up giving the player a boost while gliding.
	/* if ent.Type == entity.TypeFireworksRocket {
		ownerID, hasMetadata := ent.Metadata[entity.DataKeyOwnerID]
		metadata, hasOwner := ent.Metadata[entity.DataKeyFireworkMetadata]
		if !hasMetadata || !hasOwner {
			return
		}

		if ownerID.(int64) != int64(c.mPlayer.RuntimeId) {
			return
		}

		var flightTime uint8
		fireworkData := ((metadata.(map[string]any))["Fireworks"]).(map[string]any)
		if flight, ok := fireworkData["Flight"]; ok {
			flightTime = flight.(uint8)
		} else {
			flightTime = 1
		}

		// Firework boosters are usually already predicted client side FFS
		c.mPlayer.ACKs().Add(acknowledgement.NewGlideBoostACK(
			c.mPlayer,
			rid,
			(int64(flightTime) * 10),
			true,
		))
	} */
}

// RemoveEntity removes an entity from the entity tracker component.
func (c *EntityTrackerComponent) RemoveEntity(rid uint64) {
	delete(c.entities, rid)
}

// FindEntity searches for an entity in the entity tracker component from the given runtime ID.
func (c *EntityTrackerComponent) FindEntity(rid uint64) *entity.Entity {
	return c.entities[rid]
}

// All returns all the entities the entity tracker component is tracking.
func (c *EntityTrackerComponent) All() map[uint64]*entity.Entity {
	return c.entities
}

// MoveEntity moves an entity to the given position
func (c *EntityTrackerComponent) MoveEntity(rid uint64, tick int64, pos mgl32.Vec3, teleport bool) {
	if e, ok := c.entities[rid]; ok {
		if e.IsPlayer {
			pos[1] -= 1.62
		}
		e.RecievePosition(entity.HistoricalPosition{
			Position:     pos,
			PrevPosition: e.RecvPosition,
			Teleport:     teleport,
			Tick:         tick,
		})
	}
}

// HandleMovePlayer is a function that handles entity position updates sent with MovePlayerPacket.
func (c *EntityTrackerComponent) HandleMovePlayer(pk *packet.MovePlayer) {
	if c.ackDependent {
		c.mPlayer.ACKs().Add(acknowledgement.NewUpdateEntityPositionACK(
			c.mPlayer,
			pk.Position,
			pk.EntityRuntimeID,
			c.mPlayer.ServerTick,
			pk.Mode == packet.MoveModeTeleport,
			false,
		))
	} else {
		acknowledgement.NewUpdateEntityPositionACK(
			c.mPlayer,
			pk.Position,
			pk.EntityRuntimeID,
			c.mPlayer.ServerTick,
			pk.Mode == packet.MoveModeTeleport,
			true,
		).Run()
	}
}

// HandleMoveActorAbsolute is a function that handles entity position updates sent with MoveActorAbsolutePacket.
func (c *EntityTrackerComponent) HandleMoveActorAbsolute(pk *packet.MoveActorAbsolute) {
	if c.ackDependent {
		c.mPlayer.ACKs().Add(acknowledgement.NewUpdateEntityPositionACK(
			c.mPlayer,
			pk.Position,
			pk.EntityRuntimeID,
			c.mPlayer.ServerTick,
			utils.HasFlag(uint64(pk.Flags), packet.MoveActorDeltaFlagTeleport),
			false,
		))
	} else {
		acknowledgement.NewUpdateEntityPositionACK(
			c.mPlayer,
			pk.Position,
			pk.EntityRuntimeID,
			c.mPlayer.ServerTick,
			utils.HasFlag(uint64(pk.Flags), packet.MoveActorDeltaFlagTeleport),
			true,
		).Run()
	}
}

// SetMaxRewind sets the maximum amount of ticks that entities are allowed to be
// rewinded by.
func (c *EntityTrackerComponent) SetMaxRewind(rTicks int) {
	c.maxRewindTicks = rTicks
}

// MaxRewind returns the maximum amount of ticks that entities are allowed to be
// rewound by.
func (c *EntityTrackerComponent) MaxRewind() int {
	return c.maxRewindTicks
}

// Tick makes the entity tracker component tick all of the entities. If the player has
// full authoritative combat enabled, this is called on the "server" goroutine. On all other
// modes it is called when PlayerAuthInput is recieved.
func (c *EntityTrackerComponent) Tick(tick int64) {
	for _, e := range c.entities {
		e.Tick(tick)
	}
}
