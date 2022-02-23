package check

import (
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/justtaldevelops/oomph/settings"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// InvalidMovementA checks if the user's XZ movement is close to the predicted movement.
type InvalidMovementA struct {
	check
	lastPrediction mgl64.Vec3
}

// Name ...
func (*InvalidMovementA) Name() (string, string) {
	return "InvalidMovement", "A"
}

// Description ...
func (*InvalidMovementA) Description() string {
	return "This checks if a users horizontal movement is invalid, this can detect anything horizontal such as speed."
}

// BaseSettings ...
func (*InvalidMovementA) BaseSettings() settings.BaseSettings {
	return settings.Settings.InvalidMovement.A
}

// Process ...
func (i *InvalidMovementA) Process(processor Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		s := processor.Session()
		m := s.Movement
		diffVec := omath.AbsVec64(m.Motion.Sub(m.PreviousServerPredictedMotion))
		lastDiffVec := omath.AbsVec64(m.Motion.Sub(i.lastPrediction))
		max := 0.15
		if s.HasFlag(session.FlagCollidedHorizontally) {
			max = 0.25
		}
		if (diffVec.X() > max || diffVec.Z() > max) && (lastDiffVec.X() > max || lastDiffVec.Z() > max) && !s.HasFlag(session.FlagTeleporting) && s.Ticks.Liquid >= 10 && s.Ticks.Cobweb >= 10 {
			if i.Buff(i.updateAndGetViolationAfterTicks(processor.ClientTick(), 5)) >= 10 {
				processor.Flag(i, map[string]interface{}{"xDiff": omath.Round(diffVec.X(), 5), "zDiff": omath.Round(diffVec.Z(), 5)})
			}
		} else {
			i.Buff(-0.01)
			i.violations = math.Max(i.violations-0.01, 0)
		}
		i.lastPrediction = m.PreviousServerPredictedMotion
	}
}
