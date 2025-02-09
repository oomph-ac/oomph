package player

import (
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type EffectsComponent interface {
	// Get returns an effect from the effect component from the passed effect ID. If the effect
	// is not found, false is returned along with an empty effect.
	Get(effectID int32) (Effect, bool)
	// Add adds an effect to the effect component.
	Add(effectID int32, e Effect)
	// Remove removes an effect from the effect component, removing the effect that matches with
	// the passed effect ID.
	Remove(effectID int32)
	// Tick ticks all the effects, and removes those effects in which the duration has expired.
	Tick()
}

type Effect struct {
	Amplifier int32
	Duration  int32
}

func NewEffect(level, duration int32) Effect {
	return Effect{Amplifier: level, Duration: duration}
}

// SetEffects sets the effects component of the player.
func (p *Player) SetEffects(ec EffectsComponent) {
	p.effects = ec
}

// Effects returns the effects component of the player.
func (p *Player) Effects() EffectsComponent {
	return p.effects
}

// TODO: acks
func (p *Player) handleEffectsPacket(pk *packet.MobEffect) {
	if pk.EntityRuntimeID != p.RuntimeId {
		return
	}

	switch pk.Operation {
	case packet.MobEffectAdd, packet.MobEffectModify:
		t, ok := effect.ByID(int(pk.EffectType))
		if !ok {
			return
		}

		if _, ok := t.(effect.LastingType); ok {
			p.effects.Add(pk.EffectType, NewEffect(pk.Amplifier, pk.Duration))
		}
	case packet.MobEffectRemove:
		p.effects.Remove(pk.EffectType)
	}
}
