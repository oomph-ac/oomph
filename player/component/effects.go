package component

import (
	"github.com/oomph-ac/oomph/player"
)

type EffectsComponent struct {
	effects map[int32]player.Effect
}

// NewEffectsComponent returns a new effects component.
func NewEffectsComponent() *EffectsComponent {
	return &EffectsComponent{
		effects: make(map[int32]player.Effect),
	}
}

// Get returns an effect from the effect component from the passed effect ID. If the effect
// is not found, false is returned along with an empty effect.
func (ec *EffectsComponent) Get(effectID int32) (player.Effect, bool) {
	if e, ok := ec.effects[effectID]; ok {
		return e, true
	}
	return player.Effect{}, false
}

// Add adds an effect to the effect component.
func (ec *EffectsComponent) Add(effectID int32, e player.Effect) {
	ec.effects[effectID] = e
}

// Remove removes an effect from the effect component, removing the effect that matches with
// the passed effect ID.
func (ec *EffectsComponent) Remove(effectID int32) {
	delete(ec.effects, effectID)
}

// RemoveAll removes all effects from the effect component.
func (ec *EffectsComponent) RemoveAll() {
	for k := range ec.effects {
		delete(ec.effects, k)
	}
}

// All returns all effects that are currently active in the effect component.
func (ec *EffectsComponent) All() map[int32]player.Effect {
	return ec.effects
}

// Tick ticks all the effects, and removes those effects in which the duration has expired.
func (ec *EffectsComponent) Tick() {
	for id, e := range ec.effects {
		e.Duration--
		if e.Duration <= 0 {
			delete(ec.effects, id)
			continue
		}
		ec.effects[id] = e
	}
}
