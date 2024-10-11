package player

import "github.com/df-mc/dragonfly/server/entity/effect"

type EffectsComponent interface {
	// Get returns an effect from the effect component from the passed effect ID. If the effect
	// is not found, false is returned along with an empty effect.
	Get(effectID int32) (effect.Effect, bool)
	// Add adds an effect to the effect component.
	Add(int32, effect.Effect)
	// Delete removes an effect from the effect component, removing the effect that matches with
	// the passed effect ID.
	Delete(effectID int32)

	// Tick ticks all the effects, and removes those effects in which the duration has expired.
	Tick()
}
