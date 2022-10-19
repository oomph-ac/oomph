package check

import (
	"fmt"
	"math"

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

	xDiff, zDiff := math.Abs(xKb-p.ClientMovement()[0]), math.Abs(zKb-p.ClientMovement()[2])
	pct := (math.Hypot(p.ClientMovement()[0], p.ClientMovement()[2]) / math.Hypot(xKb, zKb)) * 100
	threshold := 5e-4

	if xDiff <= threshold && zDiff <= threshold {
		v.violations = math.Max(0, v.violations-0.2)
		v.Buff(-3)
		return false
	}

	// TODO: *This* velocity check can sometimes false for some reason. The root cause right now is still
	// unknown - but a buffer should do for now until I can investigate further. The vertical velocity
	// detection (A) does not false afaik.
	if v.Buff(1, 8) < 6 {
		return false
	}

	p.Flag(v, v.violationAfterTicks(p.ClientFrame(), 200), map[string]any{
		"pct":   fmt.Sprint(game.Round(pct, 4), "%"),
		"xDiff": game.Round(xDiff, 4),
		"zDiff": game.Round(zDiff, 4),
	})

	return false
}
