package check

import (
	"github.com/justtaldevelops/oomph/settings"
	"math"

	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

// BaseSettings ...
func (*VelocityA) BaseSettings() settings.BaseSettings {
	return settings.Settings.Velocity.A.BaseSettings
}

// Process ...
func (v *VelocityA) Process(processor Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		s := processor.Session()
		if s.Ticks.Motion != 1 || s.Ticks.Climable < 10 || s.Ticks.Cobweb < 10 || s.Ticks.Liquid < 10 || s.HasFlag(session.FlagTeleporting) || s.HasFlag(session.FlagCollidedVertically) || s.Movement.PreviousServerPredictedMotion.Y() < 0.005 {
			return
		}
		velo := s.Movement.Motion.Y() / s.Movement.PreviousServerPredictedMotion.Y()
		//processor.Debug(v, map[string]interface{}{"velo": velo})
		if velo <= 0.9999 || velo >= 1.1 {
			if v.Buff(1) >= 12 {
				processor.Flag(v, map[string]interface{}{"velo": omath.Round(velo, 6)})
			} else {
				v.Buff(-0.05)
				v.violations = math.Max(v.violations-0.025, 0)
			}
		}
	}
}
