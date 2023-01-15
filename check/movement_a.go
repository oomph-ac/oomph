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
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if p.MovementMode() != utils.ModeSemiAuthoritative {
		return false
	}

	diff := i.Delta[1] - float32(p.ServerMovement()[1])
	if math32.Abs(diff) < 0.01 {
		m.Buff(-1, 6)
		m.violations = math.Max(0, m.violations-1)
		return false
	}

	if m.Buff(1, 6) < 5.5 {
		return false
	}

	p.Flag(m, m.violationAfterTicks(p.ClientFrame(), 20), map[string]any{
		"diff": diff,
	})

	return false
}
