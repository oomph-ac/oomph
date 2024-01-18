package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDMovementA = "oomph:movement_a"

type MovementA struct {
	BaseDetection
}

func NewMovementA() *MovementA {
	d := &MovementA{}
	d.Type = "Movement"
	d.SubType = "A"

	d.Description = "Checks for deviation between server simulated movement and client movement vertically."
	d.Punishable = true

	d.MaxViolations = 30
	d.trustDuration = 20 * player.TicksPerSecond

	d.FailBuffer = 3
	d.MaxBuffer = 10
	return d
}

func (d *MovementA) ID() string {
	return DetectionIDMovementA
}

func (d *MovementA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.StepClipOffset > 0 || mDat.OnGround {
		return true
	}

	dev := math32.Abs(mDat.ClientPosition.Y() - mDat.Position.Y())
	if dev < 0.01 {
		d.Debuff(0.5)
		return true
	}

	dat := orderedmap.NewOrderedMap[string, any]()
	dat.Set("diff", game.Round32(dev, 3))
	d.Fail(p, dat)
	return true
}
