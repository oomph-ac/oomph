package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDVelocityA = "oomph:velocity_a"

type VelocityA struct {
	BaseDetection
}

func NewVelocityA() *VelocityA {
	d := &VelocityA{}
	d.Type = "Velocity"
	d.SubType = "A"

	d.Description = "Checks for vertical kb reduction."
	d.Punishable = true

	d.MaxViolations = 20
	d.trustDuration = 60 * player.TicksPerSecond

	d.FailBuffer = 2
	d.MaxBuffer = 10
	return d
}

func (d *VelocityA) ID() string {
	return DetectionIDVelocityA
}

func (d *VelocityA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if mDat.CorrectionTrustBuffer > 0 || mDat.OutgoingCorrections > 0 {
		return true
	}

	if mDat.StepClipOffset > 0 || mDat.TicksSinceKnockback > 0 || mDat.Mov.Y() < 0.03 || mDat.TicksSinceTeleport <= 20 {
		return true
	}

	pct := (mDat.ClientMov.Y() / mDat.Mov.Y()) * 100
	if pct >= 99.99 && pct <= 110 {
		d.Debuff(0.1)
		return true
	}

	data := orderedmap.NewOrderedMap[string, any]()
	data.Set("pct", game.Round32(pct, 3))
	d.Fail(p, data)
	return true
}
