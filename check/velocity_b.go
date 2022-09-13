package check

import (
	"math"

	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type VelocityB struct {
	xVel, zVel float64
	ticks      int
	basic
}

func NewVelocityB() *VelocityB {
	return &VelocityB{}
}

func (v *VelocityB) Name() (string, string) {
	return "Velocity", "B"
}

func (v *VelocityB) Description() string {
	return "This checks if the user is taking abnormal horizontal velocity."
}

func (v *VelocityB) MaxViolations() float64 {
	return 15
}

func (v *VelocityB) Process(p Processor, pk packet.Packet) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if p.TakingKnockback() {
		v.ticks = 5
		v.xVel, v.zVel = p.OldServerMovement()[0], p.OldServerMovement()[2]
	}

	if v.ticks == 0 {
		return false
	}

	defer func() {
		if v.ticks > 0 {
			v.ticks--
		}
		v.xVel, v.zVel = p.ServerMovement()[0], p.ServerMovement()[2]
	}()

	xDiff, zDiff := math.Abs(v.xVel-p.ClientMovement()[0]), math.Abs(v.zVel-p.ClientMovement()[2])
	if xDiff <= 1e-4 && zDiff <= 1e-4 {
		v.Buff(-0.1, 10)
		return false
	}

	pct := math.Hypot(p.ServerMovement()[0], p.ServerMovement()[2]) / math.Hypot(v.xVel, v.zVel)

	if v.Buff(1, 10) >= 8 {
		p.Flag(v, v.violationAfterTicks(p.ClientFrame(), 200), map[string]any{
			"pct": game.Round(pct, 4),
		})
	}

	return false
}
