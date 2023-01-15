package check

import (
	"fmt"
	"math"

	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type VelocityA struct {
	basic
}

func NewVelocityA() *VelocityA {
	return &VelocityA{}
}

func (v *VelocityA) Name() (string, string) {
	return "Velocity", "A"
}

func (v *VelocityA) Description() string {
	return "This checks if the user is taking abnormal vertical velocity."
}

func (v *VelocityA) MaxViolations() float64 {
	return 15
}

func (v *VelocityA) Process(p Processor, pk packet.Packet) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if p.MovementMode() != utils.ModeSemiAuthoritative {
		return false
	}

	if !p.TakingKnockback() {
		return false
	}

	kb := p.OldServerMovement()[1]

	if kb < 0.1 {
		return false
	}

	diff, pct := math32.Abs(kb-p.ClientMovement()[1]), (p.ClientMovement()[1]/kb)*100
	if diff <= 1e-4 {
		v.violations = math.Max(0, v.violations-0.1)
		return false
	}

	p.Flag(v, v.violationAfterTicks(p.ClientFrame(), 200), map[string]any{
		"pct": fmt.Sprint(game.Round32(pct, 4), "%"),
	})

	return false
}
