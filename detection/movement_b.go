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
	movementBMaxThreshold = 0.3
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

	d.FailBuffer = 5
	d.MaxBuffer = 7
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
	if mDat.StepClipOffset > 0 || mDat.TicksSinceTeleport <= 10 {
		return true
	}

	xDev := math32.Abs(mDat.ClientPosition.X() - mDat.Position.X())
	zDev := math32.Abs(mDat.ClientPosition.Z() - mDat.Position.Z())

	if xDev < movementBThreshold || zDev < movementBThreshold {
		d.Debuff(1)
		return true
	}

	data := orderedmap.NewOrderedMap[string, any]()
	data.Set("xDiff", game.Round32(xDev, 3))
	data.Set("zDiff", game.Round32(zDev, 3))
	d.Fail(p, data)

	// If the deviation is higher than the max threshold, we should punish the player for each time
	// their movement exceeds the threshold.
	count := float32(0)
	for hz := math32.Max(xDev, zDev); hz >= movementBMaxThreshold && count <= 3; hz -= movementBMaxThreshold {
		count++
		d.Fail(p, data)
	}

	return true
}
