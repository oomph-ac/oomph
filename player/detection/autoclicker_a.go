package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type AutoclickerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_AutoclickerA(p *player.Player) *AutoclickerA {
	d := &AutoclickerA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    4,
			MaxBuffer:     4,
			MaxViolations: 20,
			TrustDuration: -1,
		},
	}
	p.Clicks().AddLeftHook(d.hookLeft)
	p.Clicks().AddRightHook(d.hookRight)
	return d
}

func (*AutoclickerA) Type() string {
	return TypeAutoclicker
}

func (*AutoclickerA) SubType() string {
	return "A"
}

func (*AutoclickerA) Description() string {
	return "Checks if the player is clicking above the limit set in Oomph's configuration."
}

func (*AutoclickerA) Punishable() bool {
	return true
}

func (d *AutoclickerA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *AutoclickerA) Detect(pk packet.Packet) {}

func (d *AutoclickerA) hookLeft() {
	limit := d.mPlayer.Opts().Combat.LeftCPSLimit
	if d.mPlayer.InputMode == packet.InputModeTouch {
		limit = d.mPlayer.Opts().Combat.LeftCPSLimitMobile
	}
	if cps := d.mPlayer.Clicks().CPSLeft(); cps > limit {
		d.mPlayer.FailDetection(d, "left_cps", cps)
	}
}

func (d *AutoclickerA) hookRight() {
	limit := d.mPlayer.Opts().Combat.RightCPSLimit
	if d.mPlayer.InputMode == packet.InputModeTouch {
		limit = d.mPlayer.Opts().Combat.RightCPSLimitMobile
	}
	if cps := d.mPlayer.Clicks().CPSRight(); cps > limit {
		d.mPlayer.FailDetection(d, "right_cps", cps)
	}
}
