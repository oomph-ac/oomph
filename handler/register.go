package handler

import "github.com/oomph-ac/oomph/player"

// RegisterHandlers registers all handlers to the player. They are registered
// in order of priority, so that handlers registered first are called first.
func RegisterHandlers(p *player.Player) {
	p.RegisterHandler(NewLatencyHandler())

	p.RegisterHandler(NewGamemodeHandler())
	p.RegisterHandler(NewEffectsHandler())

	p.RegisterHandler(NewChunksHandler())
	p.RegisterHandler(NewMovementHandler())

	p.RegisterHandler(NewCombatHandler())
	p.RegisterHandler(NewEntityHandler())

	acks := NewAcknowledgementHandler()
	acks.LegacyMode = p.Conn().Protocol().ID() <= player.GameVersion1_20_0
	p.RegisterHandler(acks)
}
