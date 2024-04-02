package detection

import (
	"fmt"

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
	if mDat.StepClipOffset > 0 || mDat.TicksSinceKnockback > 0 || (mDat.Mov.X() < 0.005 && mDat.Mov.Z() < 0.005) || mDat.TicksSinceTeleport <= 20 {
		return true
	}

	pct := math32.Hypot(mDat.ClientMov.X(), mDat.ClientMov.Z()) / math32.Hypot(mDat.Mov.X(), mDat.Mov.Z()) * 100
	if pct < 99.25 || pct > 150 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("pct", game.Round32(pct, 3))
		d.Fail(p, data)
		return true
	}

	fmt.Println(pct)
	d.Debuff(1.0)
	return true
}
