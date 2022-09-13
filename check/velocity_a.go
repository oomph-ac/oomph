package check

import (
	"fmt"
	"math"

	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type VelocityA struct {
	knockback float64
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

	if p.TakingKnockback() {
		v.knockback = p.OldServerMovement()[1]
	}

	if v.knockback < 0.1 {
		return false
	}

	defer func() {
		v.knockback = p.ServerMovement()[1]
	}()

	diff, pct := math.Abs(v.knockback-p.ClientMovement()[1]), p.ClientMovement()[1]/v.knockback
	if diff <= 1e-4 {
		v.Buff(-0.1, 10)
		return false
	}

	if v.Buff(1, 10) >= 8 {
		p.Flag(v, v.violationAfterTicks(p.ClientFrame(), 200), map[string]any{
			"pct": fmt.Sprint(game.Round(pct, 4), "%"),
		})
	}

	return false
}
