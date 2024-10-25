package component

import "github.com/oomph-ac/oomph/player"

// RegisterAll registers the components for the given player.
func RegisterAll(p *player.Player) {
	p.SetEffects(NewEffectsComponent())
	p.SetACKs(NewACKComponent(p))
	p.SetEntityTracker(NewEntityTrackerComponent(p))
	p.SetMovement(NewAuthoritativeMovementComponent(p))
	p.SetWorldUpdater(NewWorldUpdaterComponent(p))
}
