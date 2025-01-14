package component

import "github.com/oomph-ac/oomph/player"

// Register registers the components for the given player.
func Register(p *player.Player) {
	p.SetCombat(NewAuthoritativeCombatComponent(p, false))
	p.SetClientCombat(NewAuthoritativeCombatComponent(p, true))

	p.SetEntityTracker(NewEntityTrackerComponent(p, false))
	p.SetClientEntityTracker(NewEntityTrackerComponent(p, true))

	p.SetEffects(NewEffectsComponent())
	p.SetACKs(NewACKComponent(p))
	p.SetMovement(NewAuthoritativeMovementComponent(p))
	p.SetWorldUpdater(NewWorldUpdaterComponent(p))
	p.SetGamemodeHandle(NewGamemodeComponent(p))
	p.SetInventory(NewInventoryComponent(p))
}
