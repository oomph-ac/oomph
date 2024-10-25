package player

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// GamemodeComponent is a component that handles gamemode updates for a player.
type GamemodeComponent interface {
	// Handle lets the gamemode component handle the gamemode update for the member player.
	Handle(pk *packet.SetPlayerGameType)
}

func (p *Player) SetGamemodeHandle(c GamemodeComponent) {
	p.gamemodeHandle = c
}

func (p *Player) GamemodeHandle() GamemodeComponent {
	return p.gamemodeHandle
}
