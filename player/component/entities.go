package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// EntityTrackerComponent is a component that handles entities that the member player is
// viewing on their screen.
type EntityTrackerComponent struct {
	mPlayer  *player.Player
	entities map[uint64]*entity.Entity
}

func NewEntityTrackerComponent(p *player.Player) *EntityTrackerComponent {
	return &EntityTrackerComponent{
		mPlayer:  p,
		entities: make(map[uint64]*entity.Entity),
	}
}

// AddEntity adds an entity to the entity tracker component.
func (c *EntityTrackerComponent) AddEntity(rid uint64, ent *entity.Entity) {
	c.entities[rid] = ent
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
	if oconfig.Combat().FullAuthoritative {
		c.MoveEntity(pk.EntityRuntimeID, c.mPlayer.ServerTick, pk.Position, pk.Mode == packet.MoveModeTeleport)
		return
	}
	c.mPlayer.ACKs().Add(acknowledgement.NewEntityPositionACK(
		c.mPlayer,
		pk.Position,
		pk.EntityRuntimeID,
		pk.Mode == packet.MoveModeTeleport,
	))
}

// HandleMoveActorAbsolute is a function that handles entity position updates sent with MoveActorAbsolutePacket.
func (c *EntityTrackerComponent) HandleMoveActorAbsolute(pk *packet.MoveActorAbsolute) {
	if oconfig.Combat().FullAuthoritative {
		c.MoveEntity(pk.EntityRuntimeID, c.mPlayer.ServerTick, pk.Position, utils.HasFlag(uint64(pk.Flags), packet.MoveActorDeltaFlagTeleport))
		return
	}
	c.mPlayer.ACKs().Add(acknowledgement.NewEntityPositionACK(
		c.mPlayer,
		pk.Position,
		pk.EntityRuntimeID,
		utils.HasFlag(uint64(pk.Flags), packet.MoveActorDeltaFlagTeleport),
	))
}

// Tick makes the entity tracker component tick all of the entities. If the player has
// full authoritative combat enabled, this is called on the "server" goroutine. On all other
// modes it is called when PlayerAuthInput is recieved.
func (c *EntityTrackerComponent) Tick(tick int64) {
	for _, e := range c.entities {
		e.Tick(tick)
	}
}
