package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDSpeedA = "oomph:speed_a"

type SpeedA struct {
	BaseDetection
}

func NewSpeedA() *SpeedA {
	d := &SpeedA{}
	d.Type = "Speed"
	d.SubType = "A"

	d.Description = "Checks for frictionless movement in the air."
	d.Punishable = true

	d.FailBuffer = 2
	d.MaxBuffer = 4

	d.MaxViolations = 10
	d.trustDuration = 5 * player.TicksPerSecond

	d.Settings = orderedmap.NewOrderedMap[string, any]()
	d.Settings.Set("threshold", 0.00001)
	d.Settings.Set("revert", false)
	return d
}

func (d *SpeedA) ID() string {
	return DetectionIDSpeedA
}

func (d *SpeedA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.OnGround || mDat.Climb || mDat.StepClipOffset > 0 || mDat.Immobile {
		return true
	}

	predDiff := game.AbsVec32(mDat.ClientPosition.Sub(mDat.Position))
	if predDiff.X() < 0.05 && predDiff.Z() < 0.05 {
		d.Debuff(0.5)
		return true
	}

	var jmf float32 = 0.02
	if mDat.Sprinting {
		jmf = 0.026
	}

	diff := game.AbsVec32(mDat.Mov.Sub(mDat.ClientMov))
	prev := math32.Hypot(mDat.PrevClientMov.X(), mDat.PrevClientMov.Z())
	curr := math32.Hypot(mDat.ClientMov.X(), mDat.ClientMov.Z())
	cPredDiff := curr - ((prev * 0.91) + jmf)

	if (diff.X() > 0.005 || diff.Z() > 0.005) && float64(cPredDiff) > d.Settings.GetOrDefault("threshold", 0.00001).(float64) {
		if d.Settings.GetOrDefault("revert", false).(bool) {
			mDat.CorrectMovement(p)
		}

		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("cDiff", game.Round32(cPredDiff, 4))
		data.Set("sDiff", game.Round32(diff.Len(), 4))
		d.Fail(p, data)
		return true
	}

	d.Debuff(0.05)
	return true
}
