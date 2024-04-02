package handler

import (
	"github.com/oomph-ac/oomph/player"
)

// RegisterHandlers registers all handlers to the player. They are registered
// in order of priority, so that handlers registered first are called first.
func RegisterHandlers(p *player.Player) {
	p.RegisterHandler(NewLatencyHandler())

	p.RegisterHandler(NewGamemodeHandler())
	p.RegisterHandler(NewEffectsHandler())

	p.RegisterHandler(NewChunksHandler())
	p.RegisterHandler(NewMovementHandler())

	p.RegisterHandler(NewEntityHandler())
	p.RegisterHandler(NewCombatHandler())

	acks := NewAcknowledgementHandler()
	acks.LegacyMode = p.Version <= player.GameVersion1_20_0
	p.RegisterHandler(acks)
}
