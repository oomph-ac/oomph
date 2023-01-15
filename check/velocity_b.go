package check

import (
	"fmt"
	"math"

	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type VelocityB struct {
	basic
}

func NewVelocityB() *VelocityB {
	return &VelocityB{}
}

func (v *VelocityB) Name() (string, string) {
	return "Velocity", "B"
}

func (v *VelocityB) Description() string {
	return "This checks if the user is taking abnormal horizontal velocity."
}

func (v *VelocityB) MaxViolations() float64 {
	return 15
}

func (v *VelocityB) Process(p Processor, pk packet.Packet) bool {
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

	xKb, zKb := p.OldServerMovement()[0], p.OldServerMovement()[2]
	if xKb < 0.001 && zKb < 0.001 {
		return false
	}

	pct := (math32.Hypot(p.ClientMovement()[0], p.ClientMovement()[2]) / math32.Hypot(xKb, zKb)) * 100

	if math32.Abs(100.0-pct) <= 0.1 {
		v.violations = math.Max(0, v.violations-0.2)
		v.Buff(-0.2, 5)
		return false
	}

	if v.Buff(1, 5) < 3 {
		return false
	}

	p.Flag(v, v.violationAfterTicks(p.ClientFrame(), 200), map[string]any{
		"pct": fmt.Sprint(game.Round32(pct, 4), "%"),
	})

	return false
}
