package detection

import (
	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type AimA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_AimA(p *player.Player) *AimA {
	return &AimA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 5,
			MaxBuffer:  5,

			MaxViolations: 20,
		},
	}
}

func (*AimA) Type() string {
	return TypeAim
}

func (*AimA) SubType() string {
	return "A"
}

func (*AimA) Description() string {
	return "Checks for rounded yaw deltas."
}

func (*AimA) Punishable() bool {
	return true
}

func (d *AimA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *AimA) Detect(pk packet.Packet) {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	// This check will only apply to players rotating their camera with a mouse.
	if input.InputMode != packet.InputModeMouse {
		return
	}

	if d.mPlayer.Movement().XCollision() || d.mPlayer.Movement().ZCollision() { // why does this always false ROTATION checks??!!!
		return
	}

	yawDelta := math32.Abs(d.mPlayer.Movement().RotationDelta().Z())
	if yawDelta < 1e-3 {
		return
	}

	roundedHeavy, roundedLight := game.Round32(yawDelta, 1), game.Round32(yawDelta, 5)
	diff := math32.Abs(roundedLight - roundedHeavy)

	d.mPlayer.Dbg.Notify(
		player.DebugModeAimA,
		true,
		"r1=%f r2=%f diff=%f",
		roundedHeavy,
		roundedLight,
		diff,
	)

	if diff <= 3e-5 {
		d.mPlayer.FailDetection(d, "yD", game.Round32(yawDelta, 3))
		return
	} else {
		d.mPlayer.PassDetection(d, 0.1)
	}
}
