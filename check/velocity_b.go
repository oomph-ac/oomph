package check

import (
	"math"

	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// VelocityB checks if a player is taking an abnormal amount of vertical knockback.
type VelocityB struct {
	check
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
func (*VelocityB) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*VelocityB) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (v *VelocityB) Process(processor Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		s := processor.Session()
		m := s.Movement
		if s.Ticks.Motion == 1 && math.Abs(m.PreviousServerPredictedMotion.X()) > 0.01 && math.Abs(m.PreviousServerPredictedMotion.Z()) > 0.01 {
			xVal := m.Motion.X() / m.PreviousServerPredictedMotion.X()
			zVal := m.Motion.Z() / m.PreviousServerPredictedMotion.Z()
			if ((xVal <= 0.9999 && zVal <= 0.9999) || (xVal >= 1.5 || zVal >= 1.5)) && !s.HasFlag(session.FlagTeleporting) {
				if v.Buff(v.updateAndGetViolationAfterTicks(processor.ClientTick(), 400)) >= 3 {
					processor.Flag(v, map[string]interface{}{"x": omath.Round(xVal, 6), "z": omath.Round(zVal, 6)})
				}
			} else {
				v.Buff(-0.1)
				v.violations = math.Max(v.violations-0.05, 0)
			}
		}
	}
}
