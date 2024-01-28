package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDVelocityB = "oomph:velocity_b"

type VelocityB struct {
	BaseDetection
}

func NewVelocityB() *VelocityB {
	d := &VelocityB{}
	d.Type = "Velocity"
	d.SubType = "B"

	d.Description = "Checks for horizontal kb reduction."
	d.Punishable = true

	d.MaxViolations = 20
	d.trustDuration = 60 * player.TicksPerSecond

	d.FailBuffer = 2
	d.MaxBuffer = 10
	return d
}

func (d *VelocityB) ID() string {
	return DetectionIDVelocityB
}

func (d *VelocityB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.StepClipOffset > 0 || mDat.TicksSinceKnockback > 0 || mDat.Mov.Y() < 0.05 || mDat.TicksSinceTeleport <= 3 {
		return true
	}

	xPct := (mDat.ClientMov.X() / mDat.Mov.X()) * 100
	zPct := (mDat.ClientMov.Z() / mDat.Mov.Z()) * 100

	if (100.0-xPct > 0.75 || xPct-100.0 > 5.0) && (100.0-zPct > 0.75 || zPct-100.0 > 5.0) {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("pct", game.Round32(xPct, 3))
		d.Fail(p, data)
		return true
	}

	d.Debuff(0.1)
	return true
}
