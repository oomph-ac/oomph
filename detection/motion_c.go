package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDMotionC = "oomph:motion_c"

type MotionC struct {
	BaseDetection
}

func NewMotionC() *MotionC {
	d := &MotionC{}
	d.Type = "Motion"
	d.SubType = "C"

	d.Description = "Checks for an invalid boost in motion on the Y-axis."
	d.Punishable = true

	d.FailBuffer = 1.5
	d.MaxBuffer = 4.5

	d.MaxViolations = 10
	d.trustDuration = -1

	d.Settings = orderedmap.NewOrderedMap[string, any]()
	// we set revert by default to true here, because movement cheats may
	// use this tactic at a not-so-often interval to where the buffer
	// of other detections are not reached to flag the player.
	d.Settings.Set("revert", true)
	return d
}

func (d *MotionC) ID() string {
	return DetectionIDMotionC
}

func (d *MotionC) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.CorrectionTrustBuffer > 0 || mDat.OutgoingCorrections > 0 || mDat.Climb ||
		mDat.ClientMov.Len() <= 1e-7 || mDat.TicksSinceTeleport < mDat.TeleportTicks() ||
		math32.Abs(mDat.ClientMov.Y()) < 0.1 || mDat.KnownInsideBlock {
		return true
	}

	// First, check if the current position of the player is within the predicted range of oomph's movement.
	diff := math32.Abs(mDat.Position.Y() - mDat.ClientPosition.Y())
	if diff < 0.1 {
		d.Buffer = 0
		return true
	}

	// Check to see if the player has had a significant boost in motion.
	mChange := math32.Abs(mDat.ClientMov.Y() - mDat.PrevClientMov.Y())
	yMovOppositeToServer := (mDat.PrevClientMov.Y() < 0 && mDat.ClientMov.Y() > 0 && mDat.Mov.Y() < 0) ||
		(mDat.PrevClientMov.Y() > 0 && mDat.ClientMov.Y() < 0 && mDat.Mov.Y() > 0)

	if mChange >= 0.1 || yMovOppositeToServer {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("accel", game.Round32(mChange, 3))
		d.Fail(p, data)

		if d.Settings.GetOrDefault("revert", true).(bool) {
			mDat.RevertMovement(p, mDat.Position)
		}
		return true
	}

	d.Debuff(0.05)
	return true
}
