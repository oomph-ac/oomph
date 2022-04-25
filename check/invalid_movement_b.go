package check

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// InvalidMovementB checks if the user's vertical movement is close to the predicted movement.
type InvalidMovementB struct {
	lastPrediction mgl64.Vec3
	basic
}

// NewInvalidMovementB creates a new InvalidMovementB check.
func NewInvalidMovementB() *InvalidMovementB {
	return &InvalidMovementB{}
}

// Name ...
func (*InvalidMovementB) Name() (string, string) {
	return "InvalidMovement", "B"
}

// Description ...
func (*InvalidMovementB) Description() string {
	return "This checks if a users vertical movement is invalid. This can detect anything vertical, such as flight."
}

// MaxViolations ...
func (i *InvalidMovementB) MaxViolations() float64 {
	return 50
}

// Process ...
func (i *InvalidMovementB) Process(p Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		diff := math.Abs(p.Motion().Y() - p.PreviousServerPredictedMotion().Y())
		lastDiff := math.Abs(p.Motion().Y() - i.lastPrediction.Y())
		if diff > 0.01 && lastDiff > 0.01 && !p.Teleporting() && p.LiquidTicks() >= 10 && p.CobwebTicks() >= 10 {
			if i.Buff(1, 15) >= 10 {
				p.Flag(i, i.violationAfterTicks(p.ClientTick(), 5), map[string]interface{}{
					"Y Prediction": game.Round(p.PreviousServerPredictedMotion().Y(), 5),
					"Y Movement":   game.Round(p.Motion().Y(), 5),
					"Current Diff": game.Round(diff, 5),
					"Last Diff":    game.Round(lastDiff, 5),
				})
			}
		} else {
			i.Buff(-0.02)
			i.violations = math.Max(i.violations-0.02, 0)
		}
		i.lastPrediction = p.PreviousServerPredictedMotion()
	}
}
