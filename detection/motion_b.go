package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDMotionB = "oomph:motion_b"

type MotionB struct {
	BaseDetection
}

func NewMotionB() *MotionB {
	d := &MotionB{}
	d.Type = "Motion"
	d.SubType = "B"

	d.Description = "Checks for an invalid delta field in PlayerAuthInput packets."
	d.Punishable = true

	d.FailBuffer = 3
	d.MaxBuffer = 6

	d.MaxViolations = 10
	d.trustDuration = -1

	d.Settings = orderedmap.NewOrderedMap[string, any]()
	d.Settings.Set("revert", false)
	return d
}

func (d *MotionB) ID() string {
	return DetectionIDMotionB
}

func (d *MotionB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.CorrectionTrustBuffer > 0 || mDat.StepClipOffset > 0 || mDat.Climb ||
		mDat.ClientMov.Len() <= 1e-7 || mDat.TicksSinceTeleport < mDat.TeleportTicks() ||
		mDat.StepClipOffset > 0 {
		return true
	}

	// Check if the client is deviating from the server's movement before running this detection.
	diff := game.AbsVec32(mDat.Position.Sub(mDat.ClientPosition))
	if diff.X() <= 0.01 && diff.Z() <= 0.01 {
		d.Debuff(0.5)
		return true
	}

	predicted := mDat.ClientMov.Mul(mDat.Friction)
	diff = game.AbsVec32(predicted.Sub(input.Delta))

	if diff.X() < 0.1 && diff.Z() < 0.1 {
		d.Debuff(0.05)
		return true
	}

	data := orderedmap.NewOrderedMap[string, any]()
	data.Set("dX", game.Round32(diff.X(), 3))
	data.Set("dZ", game.Round32(diff.Z(), 3))
	d.Fail(p, data)

	if d.Settings.GetOrDefault("revert", false).(bool) {
		mDat.RevertMovement(p, mDat.Position)
	}
	return true
}
