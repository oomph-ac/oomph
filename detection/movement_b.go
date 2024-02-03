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
	DetectionIDMovementB  = "oomph:movement_b"
	movementBThreshold    = 0.01
	movementBMaxThreshold = 0.1
)

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

	if xDev < movementBThreshold || zDev < movementBThreshold {
		d.Debuff(0.5)
		return true
	}

	data := orderedmap.NewOrderedMap[string, any]()
	data.Set("xDiff", game.Round32(xDev, 3))
	data.Set("zDiff", game.Round32(zDev, 3))
	d.Fail(p, data)

	// If the deviation is higher than the max threshold, we should punish the player for each time
	// their movement exceeds the threshold.
	for x := xDev; x >= movementBMaxThreshold; x -= movementBMaxThreshold {
		d.Fail(p, data)
	}
	for z := zDev; z >= movementBMaxThreshold; z -= movementBMaxThreshold {
		d.Fail(p, data)
	}

	return true
}
