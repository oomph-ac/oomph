package check

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// VelocityA checks if a player is taking an abnormal amount of vertical knock-back.
type VelocityA struct {
	basic
}

// NewVelocityA creates a new VelocityA check.
func NewVelocityA() *VelocityA {
	return &VelocityA{}
}

// Name ...
func (*VelocityA) Name() (string, string) {
	return "Velocity", "A"
}

// Description ...
func (*VelocityA) Description() string {
	return "This checks if a player is taking an abnormal amount of vertical knock-back."
}

// MaxViolations ...
func (*VelocityA) MaxViolations() float64 {
	return 15
}

// Process ...
func (v *VelocityA) Process(p Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		if p.MotionTicks() == 1 && p.ClimbableTicks() >= 10 && p.CobwebTicks() >= 10 && p.LiquidTicks() >= 10 &&
			!p.Teleporting() && !p.CollidedVertically() && p.PreviousServerPredictedMotion().Y() >= 0.005 {

			vel := p.Motion().Y() / p.PreviousServerPredictedMotion().Y()
			if vel <= 0.9999 || vel >= 1.1 {
				if v.Buff(1) >= 12 {
					p.Flag(v, v.updateAndGetViolationAfterTicks(p.ClientTick(), 100), map[string]interface{}{
						"Velocity": game.Round(vel, 6),
					})
				} else {
					v.Buff(-0.05)
					v.violations = math.Max(v.violations-0.025, 0)
				}
			}
		}
	}
}
