package check

import (
	"math"

	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type MovementB struct {
	basic
}

func NewMovementB() *MovementB {
	return &MovementB{}
}

func (*MovementB) Name() (string, string) {
	return "Movement", "B"
}

func (*MovementB) Description() string {
	return "This checks if a player's horizontal movement is invalid."
}

func (*MovementB) MaxViolations() float64 {
	return 30
}

func (m *MovementB) Process(p Processor, pk packet.Packet) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if p.MovementMode() != utils.ModeSemiAuthoritative {
		return false
	}

	if p.CanExemptMovementValidation() {
		return false
	}

	diffX := p.Entity().Position().X() - p.ServerPosition().X()
	diffZ := p.Entity().Position().Z() - p.ServerPosition().Z()

	if math32.Abs(diffX) < 0.05 && math32.Abs(diffZ) < 0.05 {
		m.Buff(-1, 10)
		m.violations = math.Max(0, m.violations-0.005)
		return false
	}

	if m.Buff(1, 10) < 5 {
		return false
	}

	p.Flag(m, m.violationAfterTicks(p.ClientFrame(), 200), map[string]any{
		"diffX": diffX,
		"diffZ": diffZ,
	})

	p.ResetServerMovement()
	p.ResetServerPosition()

	return false
}
