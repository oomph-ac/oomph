package acknowledgement

import (
	"github.com/oomph-ac/oomph/player"
)

type IdentifierNextStatus struct {
	mPlayer *player.Player
}

func NewIdentifierNextStatusACK(p *player.Player) *IdentifierNextStatus {
	return &IdentifierNextStatus{
		mPlayer: p,
	}
}

func (ack *IdentifierNextStatus) Run() {
	ack.mPlayer.Identifier().StartTimeout(100)
}
