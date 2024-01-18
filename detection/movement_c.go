package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDMovementC = "oomph:movement_b"

type MovementC struct {
	BaseDetection
}

func NewMovementC() *MovementC {
	d := &MovementC{}
	d.Type = "Movement"
	d.SubType = "C"

	d.Description = "Checks if a player is jumping in the air.."
	d.Punishable = true

	d.MaxViolations = 2
	d.trustDuration = -1

	d.FailBuffer = 2
	d.MaxBuffer = 3
	return d
}

func (d *MovementC) ID() string {
	return DetectionIDMovementC
}

func (d *MovementC) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.TicksSinceTeleport == -1 {
		return true
	}

	// If the player is not jumping, we don't need to run this check.
	if !mDat.Jumping {
		return true
	}

	// If the player has been off ground for at least 10 ticks and is jumping in the air, flag them.
	if mDat.OffGroundTicks >= 10 {
		d.Fail(p, orderedmap.NewOrderedMap[string, any]())
		return true
	}

	d.Debuff(1.0)
	return true
}
