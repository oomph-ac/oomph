package player

import (
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/justtaldevelops/oomph/game"
)

// SetEffect adds or overrides the given effect to the player.
func (p *Player) SetEffect(id int32, eff effect.Effect) {
	p.effectsMu.Lock()
	p.effects[id] = eff
	p.effectsMu.Unlock()
}

// Effect returns the effect with the given ID.
func (p *Player) Effect(id int32) (effect.Effect, bool) {
	p.effectsMu.Lock()
	eff, ok := p.effects[id]
	p.effectsMu.Unlock()
	return eff, ok
}

// RemoveEffect removes the effect with the given ID.
func (p *Player) RemoveEffect(id int32) {
	p.effectsMu.Lock()
	delete(p.effects, id)
	p.effectsMu.Unlock()
}

// tickEffects ticks all the player's effects.
func (p *Player) tickEffects() {
	p.effectsMu.Lock()
	defer p.effectsMu.Unlock()

	for i, eff := range p.effects {
		eff = eff.TickDuration()
		if eff.Duration() <= 0 {
			delete(p.effects, i)
			continue
		}

		switch eff.Type().(type) {
		case effect.JumpBoost:
			p.jumpVelocity = game.DefaultJumpMotion + (float64(eff.Level()) / 10)
		case effect.SlowFalling:
			p.gravity = game.SlowFallingGravity
		case effect.Speed:
			p.speed += 0.02 * float64(eff.Level())
		case effect.Slowness:
			// TODO: Correctly account for when both slowness and speed effects are applied.
			p.speed -= 0.015 * float64(eff.Level())
		}
		p.effects[i] = eff
	}
}
