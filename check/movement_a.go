package check

import (
	"math"

	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type MovementA struct {
	basic
}

const movementVerticalThreshold = 0.0784 // 0.98 * 0.08

func NewMovementA() *MovementA {
	return &MovementA{}
}

func (*MovementA) Name() (string, string) {
	return "Movement", "A"
}

func (*MovementA) Description() string {
	return "This checks if a player's vertical movement is invalid."
}

func (*MovementA) MaxViolations() float64 {
	return 60
}

func (m *MovementA) Process(p Processor, pk packet.Packet) bool {
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

	diff := p.Entity().Position().Y() - p.ServerPosition().Y()
	if math32.Abs(diff) < movementVerticalThreshold {
		m.Buff(-0.02, 10)
		m.violations = math.Max(0, m.violations-0.005)
		return false
	}

	if m.Buff(1, 10) < 7 {
		return false
	}

	p.Flag(m, m.violationAfterTicks(p.ClientFrame(), 200), map[string]any{
		"diff": diff,
	})

	return false
}
