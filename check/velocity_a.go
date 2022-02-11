package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// VelocityA checks if a player is taking an abnormal amount of vertical knockback.
type VelocityA struct {
	check
}

// Name ...
func (*VelocityA) Name() (string, string) {
	return "Velocity", "A"
}

// Description ...
func (*VelocityA) Description() string {
	return "This checks if a player is taking an abnormal amount of vertical knockback."
}

// MaxViolations ...
func (*VelocityA) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*VelocityA) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (v *VelocityA) Process(processor Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		s := processor.Session()
		if s.Ticks.Motion != 1 || s.Ticks.Climable < 10 || s.Ticks.Cobweb < 10 || s.Ticks.Liquid < 10 || s.HasFlag(session.FlagTeleporting) || s.Movement.PreviousServerPredictedMotion.Y() < 0.005 {
			return
		}
		velo := s.Movement.Motion.Y() / s.Movement.PreviousServerPredictedMotion.Y()
		processor.Debug(v, map[string]interface{}{"velo": velo})
		if velo < 0.99999 || velo > 1.1 {
			if v.Buff(1) >= 12 {
				processor.Flag(v, map[string]interface{}{"velo": velo})
			} else {
				v.Buff(-0.05)
				v.violations = math.Max(v.violations-0.025, 0)
			}
		}
	}
}
