package player

type EffectsComponent interface {
	// Get returns an effect from the effect component from the passed effect ID. If the effect
	// is not found, false is returned along with an empty effect.
	Get(effectID int32) (Effect, bool)
	// Add adds an effect to the effect component.
	Add(effectID int32, e Effect)
	// Remove removes an effect from the effect component, removing the effect that matches with
	// the passed effect ID.
	Remove(effectID int32)
	// RemoveAll removes all effects from the effect component.
	RemoveAll()
	// All returns all effects that are currently active in the effect component.
	All() map[int32]Effect
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
