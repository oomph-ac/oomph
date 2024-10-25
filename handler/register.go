package handler

import (
	"github.com/oomph-ac/oomph/player"
)

// RegisterHandlers registers all handlers to the player. They are registered
// in order of priority, so that handlers registered first are called first.
func RegisterHandlers(p *player.Player) {
	p.RegisterHandler(NewCombatHandler())
	p.RegisterHandler(NewRateLimitHandler())
}
