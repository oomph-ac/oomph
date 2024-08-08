package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDAimB = "oomph:aim_b"

type AimB struct {
	BaseDetection
}

func NewAimB() *AimB {
	d := &AimB{}
	d.Type = "Aim"
	d.SubType = "B"

	d.Description = "Checks for rounded yaw deltas."
	d.Punishable = true

	d.MaxViolations = 20
	d.trustDuration = -1

	d.FailBuffer = 1
	d.MaxBuffer = 1

	return d
}

func (AimB) ID() string {
	return DetectionIDAimB
}

func (d *AimB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	// This check will only apply to players rotating their camera with a mouse.
	if input.InputMode != packet.InputModeMouse {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.HorizontallyCollided { // why does this always false ROTATION checks??!!!
		return true
	}

	yawDelta := mDat.DeltaRotation.Z()
	prevYawDelta := mDat.PrevDeltaRotation.Z()
	if yawDelta < 1e-3 || yawDelta >= 180 || prevYawDelta < 1e-3 || prevYawDelta >= 180 {
		return true
	}

	if math32.Abs(yawDelta-prevYawDelta) < 1e-3 && math32.Abs(yawDelta-game.Round32(yawDelta, 3)) <= 1e-4 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("yD", game.Round32(yawDelta, 3))
		d.Fail(p, data)
	}

	return true
}
