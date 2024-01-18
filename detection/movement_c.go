package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
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

	d.MaxViolations = 30
	d.trustDuration = 20 * player.TicksPerSecond

	d.FailBuffer = 3
	d.MaxBuffer = 10
	return d
}

func (d *MovementC) ID() string {
	return DetectionIDMovementC
}

func (d *MovementC) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	
	// if mDat.TicksSinceTeleport == -1 { // ethan halp
		// return true
	// }
	
	if mDat.OffGroundTicks <= 10 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("off_ground_ticks", mDat.OffGroundTicks)
		d.Fail(p, data)
	}

	// If the player is not jumping, we don't need to run this check.
	if !utils.HasFlag(i.InputData, packet.InputFlagStartJumping) {
		return true
	}



	return true
}
