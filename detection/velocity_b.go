package detection

import (
	"github.com/chewxy/math32"
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

	d.FailBuffer = 4
	d.MaxBuffer = 8
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
	if mDat.CorrectionTrustBuffer > 0 || mDat.OutgoingCorrections > 0 {
		return true
	}

	if mDat.StepClipOffset > 0 || mDat.TicksSinceKnockback > 0 || (mDat.Mov.X() < 0.005 && mDat.Mov.Z() < 0.005) || mDat.TicksSinceTeleport <= 20 {
		return true
	}

	if math32.Max(math32.Abs(mDat.Mov.X()), math32.Abs(mDat.Mov.Z())) < 0.03 {
		return true
	}

	maxIsX := math32.Abs(mDat.Mov.X()) > math32.Abs(mDat.Mov.Z())
	if maxIsX {
		pct := math32.Abs(mDat.ClientMov.X()) / math32.Abs(mDat.Mov.X()) * 100
		if pct < 99.25 || pct > 150 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("pct", game.Round32(pct, 3))
			d.Fail(p, data)
		}

		d.Debuff(1.0)
		return true
	}

	pct := math32.Abs(mDat.ClientMov.Z()) / math32.Abs(mDat.Mov.Z()) * 100
	if pct < 99.25 || pct > 150 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("pct", game.Round32(pct, 3))
		d.Fail(p, data)
		return true
	}

	d.Debuff(1.0)
	return true
}
