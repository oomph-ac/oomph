package handler

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDEntities = "oomph:entities"
const DefaultEntityHistorySize = 6

// EntitiesHandler handles entities and their respective positions to the client. On AuthorityModeSemi, EntitiesHandler is able to
// replicate a 1:1 position of what the client sees, which is used for detections. On AuthorityModeComplete, EntitiesHandler uses rewind
// to determine entity positions based on the client tick, and is used for full authority over combat.
type EntitiesHandler struct {
	Entities       map[uint64]*entity.Entity
	MaxRewindTicks int
}

func NewEntityHandler() *EntitiesHandler {
	return &EntitiesHandler{
		Entities:       make(map[uint64]*entity.Entity),
		MaxRewindTicks: DefaultEntityHistorySize,
	}
}

func (h *EntitiesHandler) ID() string {
	return HandlerIDEntities
}

func (h *EntitiesHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if _, ok := pk.(*packet.PlayerAuthInput); ok && p.CombatMode == player.AuthorityModeSemi {
		h.tickEntities(p.ClientFrame)
	}

	return true
}

func (h *EntitiesHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.AddActor:
		width, height, scale := calculateBBSize(pk.EntityMetadata, 0.6, 1.8, 1.0)
		h.Add(pk.EntityRuntimeID, entity.New(
			pk.Position,
			pk.Velocity,
			h.MaxRewindTicks,
			false,
			width,
			height,
			scale,
		))
	case *packet.AddPlayer:
		width, height, scale := calculateBBSize(pk.EntityMetadata, 0.6, 1.8, 1.0)
		h.Add(pk.EntityRuntimeID, entity.New(
			pk.Position,
			pk.Velocity,
			h.MaxRewindTicks,
			true,
			width,
			height,
			scale,
		))
	case *packet.RemoveActor:
		h.Delete(uint64(pk.EntityUniqueID))
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID == p.RuntimeId {
			return true
		}

		// If the authority mode is set to AuthorityModeSemi, we need to wait for the client to acknowledge the
		// position before the entity is moved.
		if p.CombatMode == player.AuthorityModeSemi {
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
				ack.AckEntityUpdatePosition,
				pk.EntityRuntimeID,
				p.ServerTick,
				pk.Position,
				utils.HasFlag(uint64(pk.Flags), packet.MoveActorDeltaFlagTeleport),
			))
			return true
		}

		h.MoveEntity(pk.EntityRuntimeID, p.ServerTick, pk.Position, utils.HasFlag(uint64(pk.Flags), packet.MoveActorDeltaFlagTeleport))
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.RuntimeId {
			return true
		}

		// If the authority mode is set to AuthorityModeSemi, we need to wait for the client to acknowledge the
		// position before the entity is moved.
		if p.CombatMode == player.AuthorityModeSemi {
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
				ack.AckEntityUpdatePosition,
				pk.EntityRuntimeID,
				p.ServerTick,
				pk.Position,
				pk.Mode == packet.MoveModeTeleport,
			))
			return true
		}

		h.MoveEntity(pk.EntityRuntimeID, p.ServerTick, pk.Position, pk.Mode == packet.MoveModeTeleport)
	case *packet.SetActorMotion:
		return pk.EntityRuntimeID == p.RuntimeId
		/* if pk.EntityRuntimeID == p.RuntimeId {
			return true
		}

		entity := h.Find(pk.EntityRuntimeID)
		if entity == nil {
			return true
		}

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			entity.RecvVelocity = pk.Velocity
		}) */
	case *packet.SetActorData:
		e := h.Find(pk.EntityRuntimeID)
		if e == nil {
			return true
		}

		e.Width, e.Height, e.Scale = calculateBBSize(pk.EntityMetadata, e.Width, e.Height, e.Scale)
	}

	return true
}

func (h *EntitiesHandler) OnTick(p *player.Player) {
	if p.CombatMode != player.AuthorityModeComplete {
		return
	}

	h.tickEntities(p.ServerTick)
}

func (*EntitiesHandler) Defer() {
}

// Add adds an entity to the entity handler.
func (h *EntitiesHandler) Add(rid uint64, e *entity.Entity) {
	h.Entities[rid] = e
}

// Delete removes an entity from the entity handler.
func (h *EntitiesHandler) Delete(rid uint64) {
	delete(h.Entities, rid)
}

// Find returns an entity from the given runtime ID. Nil is returned if the entity does not exist.
func (h *EntitiesHandler) Find(rid uint64) *entity.Entity {
	return h.Entities[rid]
}

// MoveEntity moves an entity to the given position.
func (h *EntitiesHandler) MoveEntity(rid uint64, tick int64, pos mgl32.Vec3, teleport bool) {
	e := h.Find(rid)
	if e == nil {
		return
	}

	if e.IsPlayer {
		pos[1] -= 1.62
	}

	e.RecievePosition(entity.HistoricalPosition{
		Position: pos,
		Teleport: teleport,
		Tick:     tick,
	})
}

// tickEntities ticks all the entities in the entity handler.
func (h *EntitiesHandler) tickEntities(tick int64) {
	for _, e := range h.Entities {
		e.Tick(tick)
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
