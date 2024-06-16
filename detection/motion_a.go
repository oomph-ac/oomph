package detection

import (
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDMotionA = "oomph:motion_a"

type MotionA struct {
	BaseDetection
}

func NewMotionA() *MotionA {
	d := &MotionA{}
	d.Type = "Motion"
	d.SubType = "A"

	d.Description = "Checks if a player is jumping without pressing the jump key or is jumping in the air."
	d.Punishable = true

	d.MaxViolations = 2
	d.trustDuration = -1

	return d
}

func (d *MotionA) ID() string {
	return DetectionIDMotionA
}

func (d *MotionA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.CorrectionTrustBuffer > 0 || mDat.TicksSinceTeleport < mDat.TeleportTicks() || !mDat.Jumping {
		return true
	}

	// Check if the player is not pressing their jump key. Instances where a horizontal collision is detected
	// is exempted because the player may have auto-jump enabled.
	// We also check for invalid jumps in the air.
	if mDat.OffGroundTicks >= 10 {
		d.Fail(p, nil)
		return true
	}

	return true
}
