package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type InvMoveA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	preFlag bool
}

func New_InvMoveA(p *player.Player) *InvMoveA {
	return &InvMoveA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},
	}
}

func (*InvMoveA) Type() string {
	return TypeInvMove
}

func (*InvMoveA) SubType() string {
	return "A"
}

func (*InvMoveA) Description() string {
	return "Checks if a player is moving while moving items in their inventory."
}

func (*InvMoveA) Punishable() bool {
	return true
}

func (d *InvMoveA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *InvMoveA) Detect(pk packet.Packet) {
	if _, ok := pk.(*packet.ItemStackRequest); ok {
		d.preFlag = d.mPlayer.Movement().Impulse().LenSqr() > 0.0
	} else if _, ok := pk.(*packet.PlayerAuthInput); ok {
		if d.preFlag && d.mPlayer.Movement().Impulse().LenSqr() > 0.0 {
			d.mPlayer.FailDetection(d)
		}
		d.preFlag = false
	}
}
