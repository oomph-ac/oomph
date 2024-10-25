package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDAimA = "oomph:aim_a"

type AimA struct {
	BaseDetection
}

func NewAimA() *AimA {
	d := &AimA{}
	d.Type = "Aim"
	d.SubType = "A"

	d.Description = "Checks for rounded yaw deltas."
	d.Punishable = true

	d.MaxViolations = 20
	d.trustDuration = -1

	d.FailBuffer = 5
	d.MaxBuffer = 5

	return d
}

func (AimA) ID() string {
	return DetectionIDAimA
}

func (d *AimA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	// This check will only apply to players rotating their camera with a mouse.
	if input.InputMode != packet.InputModeMouse {
		return true
	}

	if p.Movement().XCollision() || p.Movement().ZCollision() { // why does this always false ROTATION checks??!!!
		return true
	}

	yawDelta := math32.Abs(p.Movement().RotationDelta().Z())
	if yawDelta < 1e-3 {
		return true
	}

	roundedHeavy, roundedLight := game.Round32(yawDelta, 1), game.Round32(yawDelta, 5)
	diff := math32.Abs(roundedLight - roundedHeavy)

	p.Dbg.Notify(
		player.DebugModeAimA,
		true,
		"r1=%f r2=%f diff=%f",
		roundedHeavy,
		roundedLight,
		diff,
	)

	if diff <= 3e-5 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("yD", game.Round32(yawDelta, 3))
		d.Fail(p, data)
		return true
	}

	d.Debuff(0.1)
	return true
}
