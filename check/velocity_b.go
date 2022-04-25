package check

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// VelocityB checks if a player is taking an abnormal amount of horizontal knockback.
type VelocityB struct {
	basic
}

// NewVelocityB creates a new VelocityB check.
func NewVelocityB() *VelocityB {
	return &VelocityB{}
}

// Name ...
func (*VelocityB) Name() (string, string) {
	return "Velocity", "B"
}

// Description ...
func (*VelocityB) Description() string {
	return "This checks if a player is taking an abnormal amount of horizontal knockback."
}

// MaxViolations ...
func (v *VelocityB) MaxViolations() float64 {
	return 15
}

// Process ...
func (v *VelocityB) Process(p Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		if p.MotionTicks() == 1 && math.Abs(p.PreviousServerPredictedMotion().X()) > 0.01 &&
			math.Abs(p.PreviousServerPredictedMotion().Z()) > 0.01 {
			xVal := p.Motion().X() / p.PreviousServerPredictedMotion().X()
			zVal := p.Motion().Z() / p.PreviousServerPredictedMotion().Z()
			if ((xVal <= 0.9999 && zVal <= 0.9999) || (xVal >= 1.5 || zVal >= 1.5)) && !p.Teleporting() && !p.CollidedHorizontally() {
				if v.Buff(v.violationAfterTicks(p.ClientTick(), 400)) >= 3 {
					p.Flag(v, v.violationAfterTicks(p.ClientTick(), 100), map[string]interface{}{
						"Velocity (X)": game.Round(xVal, 6),
						"Velocity (Z)": game.Round(zVal, 6)},
					)
				}
			} else {
				v.Buff(-0.1)
				v.violations = math.Max(v.violations-0.05, 0)
			}
		}
	}
}
