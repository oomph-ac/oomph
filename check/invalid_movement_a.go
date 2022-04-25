package check

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// InvalidMovementA checks if the user's XZ movement is close to the predicted movement.
type InvalidMovementA struct {
	lastPrediction mgl64.Vec3
	basic
}

// NewInvalidMovementA creates a new InvalidMovementA check.
func NewInvalidMovementA() *InvalidMovementA {
	return &InvalidMovementA{}
}

// Name ...
func (*InvalidMovementA) Name() (string, string) {
	return "InvalidMovement", "A"
}

// Description ...
func (*InvalidMovementA) Description() string {
	return "This checks if a users horizontal movement is invalid. This can detect anything horizontal, such as speed."
}

// MaxViolations ...
func (i *InvalidMovementA) MaxViolations() float64 {
	return 50
}

// Process ...
func (i *InvalidMovementA) Process(p Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.PlayerAuthInput:
		if p.Motion().LenSqr() <= 1e-10 {
			return
		}
		diffVec := game.AbsVec64(p.Motion().Sub(p.PreviousServerPredictedMotion()))
		lastDiffVec := game.AbsVec64(p.Motion().Sub(i.lastPrediction))

		max := 0.15
		if p.CollidedHorizontally() {
			max = 0.25
		}

		if (diffVec.X() > max || diffVec.Z() > max) && (lastDiffVec.X() > max || lastDiffVec.Z() > max) &&
			!p.Teleporting() && p.LiquidTicks() >= 10 && p.CobwebTicks() >= 10 {
			if i.Buff(1, 15) >= 10 {
				p.Flag(i, i.violationAfterTicks(p.ClientTick(), 5), map[string]interface{}{
					"Difference (X)": game.Round(diffVec.X(), 5),
					"Difference (Z)": game.Round(diffVec.Z(), 5),
				})
			}
		} else {
			i.Buff(-0.01)
			i.violations = math.Max(i.violations-0.01, 0)
		}
		i.lastPrediction = p.PreviousServerPredictedMotion()
	}
}
