package check

import (
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// AimAssistA checks the correlation coefficient between expected aim-bot rotation values and actual rotation values.
type AimAssistA struct {
	check
	waiting             bool
	target              uint64
	attackPos           mgl64.Vec3
	realRotationSamples []float64
	botRotationSamples  []float64
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
func (*AimAssistA) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*AimAssistA) Punishment() punishment.Punishment {
	return punishment.Ban()
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
			a.attackPos = omath.Vec32To64(data.Position)
			a.realRotationSamples, a.botRotationSamples = []float64{}, []float64{}
		}
	case *packet.PlayerAuthInput:
		if a.waiting {
			if e, ok := processor.Entity(a.target); ok {
				selfLoc := processor.Location()
				yawDiff := math.Mod(selfLoc.Rotation.Y()-selfLoc.LastRotation.Y(), 180)
				if yawDiff >= 180 {
					yawDiff = 180 - math.Mod(yawDiff, 180)
				}
				xDist, zDist := e.LastPosition.X()-a.attackPos.X(), e.LastPosition.Z()-a.attackPos.Z()
				botYaw := math.Mod(math.Atan2(zDist, xDist)/math.Pi*180-90, 180)
				botDiff := botYaw - selfLoc.LastRotation.Y()
				if botDiff >= 180 {
					botDiff = 180 - math.Mod(botDiff, 180)
				}
				if math.Abs(yawDiff) >= 0.0075 || math.Abs(botDiff) >= 0.0075 {
					a.realRotationSamples = append(a.realRotationSamples, selfLoc.Rotation.X())
					a.botRotationSamples = append(a.botRotationSamples, botYaw)
				}
				if len(a.realRotationSamples) == 40 || len(a.botRotationSamples) == 40 {
					cc := omath.CorrelationCoefficient(a.realRotationSamples, a.botRotationSamples)
					if cc > 0.99 {
						processor.Flag(a, map[string]interface{}{"correlation": omath.Round(cc, 2)})
					}
					a.realRotationSamples, a.botRotationSamples = []float64{}, []float64{}
				}
			}
			a.waiting = false
		}
	}
}
