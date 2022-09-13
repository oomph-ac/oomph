package check

import (
	"math"

	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type MovemntA struct {
	basic
}

func NewMovementA() *MovemntA {
	return &MovemntA{}
}

func (*MovemntA) Name() (string, string) {
	return "Movement", "A"
}

func (*MovemntA) Description() string {
	return "This checks if a player's vertical movement is invalid."
}

func (*MovemntA) MaxViolations() float64 {
	return 20
}

func (m *MovemntA) Process(p Processor, pk packet.Packet) bool {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if p.MovementMode() != utils.ModeSemiAuthoritative {
		return false
	}

	diff := i.Delta[1] - float32(p.ServerMovement()[1])
	if game.AbsFloat32(diff) < 0.01 {
		m.Buff(-0.5, 6)
		m.violations = math.Max(0, m.violations-0.25)
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
