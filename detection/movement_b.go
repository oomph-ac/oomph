package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDMovementB = "oomph:movement_b"

type MovementB struct {
	BaseDetection
}

func NewMovementB() *MovementB {
	d := &MovementB{}
	d.Type = "Movement"
	d.SubType = "B"

	d.Description = "Checks for deviation between server simulated movement and client movement horizontally."
	d.Punishable = true

	d.MaxViolations = 30
	d.trustDuration = 20 * player.TicksPerSecond

	d.FailBuffer = 3
	d.MaxBuffer = 10
	return d
}

func (d *MovementB) ID() string {
	return DetectionIDMovementB
}

func (d *MovementB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.StepClipOffset > 0 {
		return true
	}

	xDev := math32.Abs(mDat.ClientPosition.X() - mDat.Position.X())
	zDev := math32.Abs(mDat.ClientPosition.Z() - mDat.Position.Z())

	if xDev < 0.01 || zDev < 0.01 {
		d.Debuff(0.5)
		return true
	}

	dat := orderedmap.NewOrderedMap[string, any]()
	dat.Set("xDiff", game.Round32(xDev, 3))
	dat.Set("zDiff", game.Round32(zDev, 3))
	d.Fail(p, dat)
	return true
}
