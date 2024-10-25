package component

import (
	"github.com/df-mc/dragonfly/server/entity/effect"
)

type EffectsComponent struct {
	effects map[int32]effect.Effect
}

// NewEffectsComponent returns a new effects component.
func NewEffectsComponent() *EffectsComponent {
	return &EffectsComponent{
		effects: make(map[int32]effect.Effect),
	}
}

// Get returns an effect from the effect component from the passed effect ID. If the effect
// is not found, false is returned along with an empty effect.
func (ec *EffectsComponent) Get(effectID int32) (effect.Effect, bool) {
	if e, ok := ec.effects[effectID]; ok {
		return e, true
	}
	return effect.Effect{}, false
}

// Add adds an effect to the effect component.
func (ec *EffectsComponent) Add(effectID int32, e effect.Effect) {
	// If the effect component already has this effect at a higher level, reject.
	if current, ok := ec.effects[effectID]; ok && current.Level() > e.Level() {
		return
	}
	ec.effects[effectID] = e
}

// Remove removes an effect from the effect component, removing the effect that matches with
// the passed effect ID.
func (ec *EffectsComponent) Remove(effectID int32) {
	delete(ec.effects, effectID)

}

// Tick ticks all the effects, and removes those effects in which the duration has expired.
func (ec *EffectsComponent) Tick() {
	for id, e := range ec.effects {
		e = e.TickDuration()
		if e.Duration() <= 0 {
			delete(ec.effects, id)
		} else {
			ec.effects[id] = e
		}
	}
}
