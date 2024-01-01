package player

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDEntities = "oomph:entities"
const DefaultEntityHistorySize = 6

// EntityHandler handles entities and their respective positions to the client. On AuthorityModeSemi, EntityHandler is able to
// replicate a 1:1 position of what the client sees, which is used for detections. On AuthorityModeComplete, EntityHandler uses rewind
// to determine entity positions based on the client tick, and is used for full authority over combat.
type EntityHandler struct {
	Entities       map[uint64]*entity.Entity
	MaxRewindTicks int
}

func (h *EntityHandler) ID() string {
	return HandlerIDEntities
}

func (h *EntityHandler) HandleClientPacket(pk packet.Packet, p *Player) bool {
	if _, ok := pk.(*packet.PlayerAuthInput); ok && p.CombatMode == AuthorityModeSemi {
		h.tickEntities(p.serverTick)
	}

	return true
}

func (h *EntityHandler) HandleServerPacket(pk packet.Packet, p *Player) bool {
	switch pk := pk.(type) {
	case *packet.AddActor:
		h.AddEntity(pk.EntityRuntimeID, entity.New(pk.Position, pk.Velocity, h.MaxRewindTicks, false))
	case *packet.AddPlayer:
		h.AddEntity(pk.EntityRuntimeID, entity.New(pk.Position, pk.Velocity, h.MaxRewindTicks, true))
	case *packet.RemoveActor:
		h.RemoveEntity(uint64(pk.EntityUniqueID))
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID == p.runtimeId {
			return true
		}

		if pk.EntityRuntimeID == p.clientRuntimeId {
			pk.EntityRuntimeID = math.MaxUint64
		}

		// If the authority mode is set to AuthorityModeSemi, we need to wait for the client to acknowledge the
		// position before the entity is moved.
		if p.CombatMode == AuthorityModeSemi {
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
				h.moveEntity(pk.EntityRuntimeID, p.serverTick, pk.Position)
			})
			return true
		}

		h.moveEntity(pk.EntityRuntimeID, p.serverTick, pk.Position)
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.runtimeId {
			return true
		}

		if pk.EntityRuntimeID == p.clientRuntimeId {
			pk.EntityRuntimeID = math.MaxUint64
		}

		// If the authority mode is set to AuthorityModeSemi, we need to wait for the client to acknowledge the
		// position before the entity is moved.
		if p.CombatMode == AuthorityModeSemi {
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
				h.moveEntity(pk.EntityRuntimeID, p.serverTick, pk.Position)
			})
			return true
		}

		h.moveEntity(pk.EntityRuntimeID, p.serverTick, pk.Position)
	}

	return true
}

func (h *EntityHandler) OnTick(p *Player) {
	if p.CombatMode != AuthorityModeComplete {
		return
	}

	h.tickEntities(p.serverTick)
}

// AddEntity adds an entity to the entity handler.
func (h *EntityHandler) AddEntity(rid uint64, e *entity.Entity) {
	h.Entities[rid] = e
}

// Entity returns an entity from the given runtime ID. Nil is returned if the entity does not exist.
func (h *EntityHandler) Entity(rid uint64) *entity.Entity {
	return h.Entities[rid]
}

// RemoveEntity removes an entity from the entity handler.
func (h *EntityHandler) RemoveEntity(rid uint64) {
	delete(h.Entities, rid)
}

// moveEntity moves an entity to the given position.
func (h *EntityHandler) moveEntity(rid uint64, tick int64, pos mgl32.Vec3) {
	e := h.Entity(rid)
	if e == nil {
		return
	}

	e.RecievePosition(entity.HistoricalPosition{
		Position: pos,
		Tick:     tick,
	})
}

// tickEntities ticks all the entities in the entity handler.
func (h *EntityHandler) tickEntities(tick int64) {
	for rid, e := range h.Entities {
		e.Tick(tick)
		fmt.Println(tick, rid, e.Position, e.InterpolationTicks)
	}
}
