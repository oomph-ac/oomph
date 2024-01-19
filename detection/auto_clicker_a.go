package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDAutoClickerA = "oomph:auto_clicker_a"

type AutoClickerA struct {
	BaseDetection
}

func NewAutoClickerA() *AutoClickerA {
	d := &AutoClickerA{}
	d.Type = "AutoClicker"
	d.SubType = "A"

	d.Description = "Checks if a players cps is over a certain threshold."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *AutoClickerA) ID() string {
	return DetectionIDAutoClickerA
}

func (d *AutoClickerA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	return true
}
