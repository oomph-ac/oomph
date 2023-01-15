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
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if p.MovementMode() != utils.ModeSemiAuthoritative {
		return false
	}

	diffX, diffZ := i.Delta[0]-float32(p.ServerMovement()[0]), i.Delta[2]-float32(p.ServerMovement()[2])
	if math32.Abs(diffX) < 0.02 || math32.Abs(diffZ) < 0.02 {
		m.Buff(-1, 6)
		m.violations = math.Max(0, m.violations-1)
		return false
	}

	if m.Buff(1, 6) < 5.5 {
		return false
	}

	p.Flag(m, m.violationAfterTicks(p.ClientFrame(), 20), map[string]any{
		"diffX": diffX,
		"diffZ": diffZ,
	})

	return false
}
