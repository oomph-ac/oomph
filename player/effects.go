package player

import (
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/oomph-ac/oomph/game"
)

// SetEffect sets an effect into the effect map
func (p *Player) SetEffect(id int32, eff effect.Effect) {
	p.effectMu.Lock()
	p.effects[id] = eff
	p.effectMu.Unlock()
}

// Effect gets the effect from the effect map
func (p *Player) Effect(id int32) (effect.Effect, bool) {
	p.effectMu.Lock()
	eff, ok := p.effects[id]
	p.effectMu.Unlock()
	return eff, ok
}

// RemoveEffect removes an effect from the effect map
func (p *Player) RemoveEffect(id int32) {
	p.effectMu.Lock()
	delete(p.effects, id)
	p.effectMu.Unlock()
}

// tickEffects ticks the effects in the effect map. This will also remove any effects that have expired.
func (p *Player) tickEffects() {
	p.effectMu.Lock()
	defer p.effectMu.Unlock()

	for i, eff := range p.effects {
		eff = eff.TickDuration()
		if eff.Duration() <= 0 {
			delete(p.effects, i)
			continue
		}

		switch eff.Type().(type) {
		case effect.JumpBoost:
			p.mInfo.JumpVelocity = game.DefaultJumpMotion + (float32(eff.Level()) / 10)
		case effect.SlowFalling:
			p.mInfo.Gravity = game.SlowFallingGravity
		}

		p.effects[i] = eff
	}
}
