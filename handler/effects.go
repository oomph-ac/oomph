package handler

import (
	"time"

	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDEffects = "oomph:effects"

// EffectsHandler handles effects for the player.
type EffectsHandler struct {
	// Effects is a map of effects that the player has.
	Effects map[int32]effect.Effect
}

func NewEffectsHandler() *EffectsHandler {
	return &EffectsHandler{
		Effects: map[int32]effect.Effect{},
	}
}

func (EffectsHandler) ID() string {
	return HandlerIDEffects
}

func (h *EffectsHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	for id, e := range h.Effects {
		h.Effects[id] = e.TickDuration()
		if e.Duration() <= 0 {
			delete(h.Effects, id)
			continue
		}
	}

	return true
}

func (h *EffectsHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	epk, ok := pk.(*packet.MobEffect)
	if !ok {
		return true
	}

	// Don't handle the effect if it's not for the player.
	if epk.EntityRuntimeID != p.RuntimeId {
		return true
	}

	switch epk.Operation {
	case packet.MobEffectAdd, packet.MobEffectModify:
		t, ok := effect.ByID(int(epk.EffectType))
		if !ok {
			return true
		}

		e, ok := t.(effect.LastingType)
		if !ok {
			return true
		}

		h.Effects[epk.EffectType] = effect.New(e, int(epk.Amplifier)+1, time.Duration(epk.Duration*50)*time.Millisecond)
	case packet.MobEffectRemove:
		delete(h.Effects, epk.EffectType)
	}

	return true
}

func (*EffectsHandler) OnTick(p *player.Player) {
}

func (*EffectsHandler) Defer() {
}

func (h *EffectsHandler) Get(id int32) (effect.Effect, bool) {
	e, ok := h.Effects[id]
	return e, ok
}
