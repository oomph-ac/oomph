package handler

import "github.com/oomph-ac/oomph/player"

type Simulator interface {
	// Simulate starts the specified simulation for the player.
	Simulate(p *player.Player)
	// Reliable returns true if the simulator is able to simulate the player properly. This may return
	// false in instances where the simulator has not yet accounted for certain states.
	Reliable(p *player.Player) bool
}
