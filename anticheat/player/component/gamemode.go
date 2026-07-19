package component

import (
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component/acknowledgement"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type GamemodeComponent struct {
	mPlayer *player.Player
}

func NewGamemodeComponent(p *player.Player) *GamemodeComponent {
	return &GamemodeComponent{mPlayer: p}
}

func (c *GamemodeComponent) Handle(pk *packet.SetPlayerGameType) {
	c.mPlayer.ACKs().Add(acknowledgement.NewUpdateGamemodeACK(c.mPlayer, pk.GameType))
}
