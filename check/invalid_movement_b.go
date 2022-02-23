package check

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/justtaldevelops/oomph/settings"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// InvalidMovementB checks if the user's Y movement is close to the predicted movement.
type InvalidMovementB struct {
	check
	lastPrediction mgl64.Vec3
}

// Name ...
func (*InvalidMovementB) Name() (string, string) {
	return "InvalidMovement", "B"
}

// Description ...
func (*InvalidMovementB) Description() string {
	return "This checks if a users vertical movement is invalid, this can detect anything vertical such as flight."
}

// BaseSettings ...
func (*InvalidMovementB) BaseSettings() settings.BaseSettings {
	return settings.Settings.InvalidMovement.B
}

// Process ...
func (i *InvalidMovementB) Process(processor Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		s := processor.Session()
		m := s.Movement
		diff := math.Abs(m.Motion.Y() - m.PreviousServerPredictedMotion.Y())
		lastDiff := math.Abs(m.Motion.Y() - i.lastPrediction.Y())
		if diff > 0.01 && lastDiff > 0.01 && !s.HasFlag(session.FlagTeleporting) && s.Ticks.Liquid >= 10 && s.Ticks.Cobweb >= 10 {
			if i.Buff(1, 15) >= 10 {
				processor.Flag(i, i.updateAndGetViolationAfterTicks(processor.ClientTick(), 5), map[string]interface{}{"previousY": omath.Round(m.PreviousServerPredictedMotion.Y(), 5), "motionY": omath.Round(m.Motion.Y(), 5)})
			}
		} else {
			i.Buff(-0.02)
			i.violations = math.Max(i.violations-0.02, 0)
		}
		i.lastPrediction = m.PreviousServerPredictedMotion
	}
}
