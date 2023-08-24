package player

import (
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/oomph-ac/oomph/game"
)

// SetEffect sets an effect into the effect map
func (p *Player) SetEffect(id int32, eff effect.Effect) {
	p.effects.Store(id, eff)
}

// Effect gets the effect from the effect map
func (p *Player) Effect(id int32) (effect.Effect, bool) {
	v, ok := p.effects.Load(id)
	if !ok {
		return effect.Effect{}, false
	}

	return v.(effect.Effect), ok
}

// RemoveEffect removes an effect from the effect map
func (p *Player) RemoveEffect(id int32) {
	p.effects.Delete(id)
}

// tickEffects ticks the effects in the effect map. This will also remove any effects that have expired.
func (p *Player) tickEffects() {
	p.effects.Range(func(k, v any) bool {
		eff := v.(effect.Effect)
		eff = eff.TickDuration()
		if eff.Duration() <= 0 {
			p.effects.Delete(k)
			return true
		}

		switch eff.Type().(type) {
		case effect.JumpBoost:
			p.mInfo.JumpVelocity = game.DefaultJumpMotion + (float32(eff.Level()) / 10)
		case effect.SlowFalling:
			p.mInfo.Gravity = game.SlowFallingGravity
		}

		p.effects.Store(k, eff)
		return true
	})
}
