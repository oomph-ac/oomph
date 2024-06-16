package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	DetectionIDFlyA = "oomph:fly_a"
)

type FlyA struct {
	BaseDetection
}

func NewFlyA() *FlyA {
	d := &FlyA{}
	d.Type = "Fly"
	d.SubType = "A"

	d.Description = "Checks for deviation between server simulated movement and client movement vertically."
	d.Punishable = true

	d.MaxViolations = 15
	d.trustDuration = 10 * player.TicksPerSecond

	d.FailBuffer = 5
	d.MaxBuffer = 10

	d.Settings = orderedmap.NewOrderedMap[string, any]()
	d.Settings.Set("threshold", 0.03)
	d.Settings.Set("revert", false)
	return d
}

func (d *FlyA) ID() string {
	return DetectionIDFlyA
}

func (d *FlyA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.CorrectionTrustBuffer > 0 || mDat.StepClipOffset > 0 || mDat.KnownInsideBlock ||
		mDat.OffGroundTicks <= 3 {
		return true
	}

	dev := math32.Abs(mDat.Position.Y() - mDat.ClientPosition.Y())
	if float64(dev) < d.Settings.GetOrDefault("threshold", 0.03).(float64) {
		d.Debuff(1)
		return true
	}

	data := orderedmap.NewOrderedMap[string, any]()
	data.Set("diff", game.Round32(dev, 3))
	d.Fail(p, data)

	if d.Settings.GetOrDefault("revert", false).(bool) {
		mDat.RevertMovement(p, mDat.PrevPosition)
	}
	return true
}
