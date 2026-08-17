package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/anticheat/entity"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component/acknowledgement"
	"github.com/oomph-ac/oomph/anticheat/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// EntityTrackerComponent is a component that handles entities that the member player is
// viewing on their screen.
type EntityTrackerComponent struct {
	mPlayer    *player.Player
	entities   map[uint64]*entity.Entity
	runtimeIDs map[int64]uint64

	isClientTracker bool
}

func NewEntityTrackerComponent(p *player.Player, clientTracker bool) *EntityTrackerComponent {
	return &EntityTrackerComponent{
		mPlayer:    p,
		entities:   make(map[uint64]*entity.Entity),
		runtimeIDs: make(map[int64]uint64),

		isClientTracker: clientTracker,
	}
}

// AddEntity adds an entity to the entity tracker component.
func (c *EntityTrackerComponent) AddEntity(rid uint64, ent *entity.Entity) {
	if previousRID, ok := c.runtimeIDs[ent.UniqueId]; ok && previousRID != rid {
		c.RemoveEntity(previousRID)
	}
	c.RemoveEntity(rid)
	c.entities[rid] = ent
	c.runtimeIDs[ent.UniqueId] = rid
}

// RemoveEntity removes an entity from the entity tracker component.
func (c *EntityTrackerComponent) RemoveEntity(rid uint64) {
	if e, ok := c.entities[rid]; ok {
		delete(c.runtimeIDs, e.UniqueId)
		delete(c.entities, rid)
	}
}

// RemoveEntityByUniqueID ...
func (c *EntityTrackerComponent) RemoveEntityByUniqueID(uniqueID int64) {
	if rid, ok := c.runtimeIDs[uniqueID]; ok {
		delete(c.runtimeIDs, uniqueID)
		delete(c.entities, rid)
	}
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
		pos[1] -= e.NetworkOffset
		e.ReceivePosition(pos, teleport)
	}
}

// MoveEntityDelta ...
func (c *EntityTrackerComponent) MoveEntityDelta(rid uint64, tick int64, posX, posY, posZ protocol.Optional[float32], teleport bool) {
	e, ok := c.entities[rid]
	if !ok {
		return
	}
	pos, moved := e.RecvPosition, false
	if x, has := posX.Value(); has {
		pos[0], moved = x, true
	}
	if y, has := posY.Value(); has {
		pos[1], moved = y-e.NetworkOffset, true
	}
	if z, has := posZ.Value(); has {
		pos[2], moved = z, true
	}
	if !moved {
		return
	}
	e.ReceivePosition(pos, teleport)
}

// HandleMovePlayer is a function that handles entity position updates sent with MovePlayerPacket.
func (c *EntityTrackerComponent) HandleMovePlayer(pk *packet.MovePlayer) {
	if !c.isClientTracker {
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
	if !c.isClientTracker {
		c.MoveEntity(pk.EntityRuntimeID, c.mPlayer.ServerTick, pk.Position, utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport))
		return
	}
	c.mPlayer.ACKs().Add(acknowledgement.NewEntityPositionACK(
		c.mPlayer,
		pk.Position,
		pk.EntityRuntimeID,
		utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport),
	))
}

// HandleMoveActorDelta ...
func (c *EntityTrackerComponent) HandleMoveActorDelta(pk *packet.MoveActorDelta) {
	if !c.isClientTracker {
		c.MoveEntityDelta(pk.EntityRuntimeID, c.mPlayer.ServerTick, pk.PositionX, pk.PositionY, pk.PositionZ, pk.ForceMove)
		return
	}
	c.mPlayer.ACKs().Add(acknowledgement.NewEntityDeltaPositionACK(
		c.mPlayer,
		pk.PositionX,
		pk.PositionY,
		pk.PositionZ,
		pk.EntityRuntimeID,
		pk.ForceMove,
	))
}

// HandleSetActorData is a function that handles entity data updates sent with SetActorDataPacket.
func (c *EntityTrackerComponent) HandleSetActorData(pk *packet.SetActorData) {
	if e := c.FindEntity(pk.EntityRuntimeID); e != nil {
		width, height, scale := calculateBBSize(pk.EntityMetadata, e.Width, e.Height, e.Scale)
		if c.isClientTracker {
			c.mPlayer.ACKs().Add(acknowledgement.NewEntityDataACK(e, pk.EntityMetadata, width, height, scale))
		} else {
			e.UpdateMetadata(pk.EntityMetadata)
			e.Width, e.Height, e.Scale = width, height, scale
		}
	}
}

// Tick makes the entity tracker component tick all of the entities. If the player has
// full authoritative combat enabled, this is called on the "server" goroutine. On all other
// modes it is called when PlayerAuthInput is received.
func (c *EntityTrackerComponent) Tick(tick int64) {
	for rid, e := range c.entities {
		if err := e.Tick(tick); err != nil {
			c.mPlayer.Log().Error("entity tick failed", "rid", rid, "err", err)
		}
	}
}

// calculateBBSize calculates the bounding box size for an entity based on the EntityMetadata.
func calculateBBSize(data map[uint32]any, defaultWidth, defaultHeight, defaultScale float32) (width float32, height float32, s float32) {
	width = defaultWidth
	if w, ok := data[entity.DataKeyBoundingBoxWidth]; ok {
		width = w.(float32)
	}

	height = defaultHeight
	if h, ok := data[entity.DataKeyBoundingBoxHeight]; ok {
		height = h.(float32)
	}

	s = defaultScale
	if scale, ok := data[entity.DataKeyScale]; ok {
		s = scale.(float32)
	}

	return
}
