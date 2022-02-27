package check

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// AimAssistA checks the correlation coefficient between expected aim-bot rotation values and actual rotation values.
type AimAssistA struct {
	basic
	waiting             bool
	target              uint64
	attackPos           mgl64.Vec3
	realRotationSamples []float64
	botRotationSamples  []float64
}

// NewAimAssistA creates a new AimAssistA check.
func NewAimAssistA() *AimAssistA {
	return &AimAssistA{}
}

// Name ...
func (*AimAssistA) Name() (string, string) {
	return "AimAssist", "A"
}

// Description ...
func (*AimAssistA) Description() string {
	return "This checks if a player is using a cheat to assist with their aim."
}

// MaxViolations ...
func (*AimAssistA) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AimAssistA) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			if a.target == data.TargetEntityRuntimeID {
				a.waiting = true
				return
			}
			a.target = data.TargetEntityRuntimeID
			a.attackPos = game.Vec32To64(data.Position.Sub(mgl32.Vec3{0, 1.621}))
			a.realRotationSamples, a.botRotationSamples = []float64{}, []float64{}
		}
	case *packet.PlayerAuthInput:
		if a.waiting {
			if e, ok := processor.SearchEntity(a.target); ok {
				selfLoc := processor.Entity()
				yawDiff := math.Mod(selfLoc.Rotation().Y()-selfLoc.LastRotation().Y(), 180)
				if yawDiff >= 180 {
					yawDiff = 180 - math.Mod(yawDiff, 180)
				}
				xDist, zDist := e.LastPosition().X()-a.attackPos.X(), e.LastPosition().Z()-a.attackPos.Z()
				botYaw := math.Mod(math.Atan2(zDist, xDist)/math.Pi*180-90, 180)
				botDiff := botYaw - selfLoc.LastRotation().Y()
				if botDiff >= 180 {
					botDiff = 180 - math.Mod(botDiff, 180)
				}
				if math.Abs(yawDiff) >= 0.0075 || math.Abs(botDiff) >= 0.0075 {
					a.realRotationSamples = append(a.realRotationSamples, selfLoc.Rotation().X())
					a.botRotationSamples = append(a.botRotationSamples, botYaw)
				}
				if len(a.realRotationSamples) == 40 || len(a.botRotationSamples) == 40 {
					cc := game.CorrelationCoefficient(a.realRotationSamples, a.botRotationSamples)
					if cc > 0.99 {
						processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 200), map[string]interface{}{
							"Correlation": game.Round(cc, 2)},
						)
					}
					a.realRotationSamples, a.botRotationSamples = []float64{}, []float64{}
				}
			}
			a.waiting = false
		}
	}
}
