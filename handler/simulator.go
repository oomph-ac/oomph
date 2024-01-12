package handler

import "github.com/oomph-ac/oomph/player"

type Simulator interface {
	// Simulate starts the specified simulation for the player.
	Simulate(p *player.Player)
}
