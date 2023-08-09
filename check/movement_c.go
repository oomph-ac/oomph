package check

import (
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type MovementC struct {
	basic
	offGroundTicks uint64
}

func NewMovementC() *MovementC {
	return &MovementC{}
}

func (*MovementC) Name() (string, string) {
	return "Movement", "C"
}

func (*MovementC) Description() string {
	return "This checks if a player is jumping in the air."
}

func (*MovementC) MaxViolations() float64 {
	return 1
}

func (m *MovementC) Process(p Processor, pk packet.Packet) bool {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	// Only run movement checks if the movement mode is set to semi authoritative.
	if p.MovementMode() != utils.ModeSemiAuthoritative {
		return false
	}

	defer func() {
		if p.OnGround() {
			m.offGroundTicks = 0
			return
		}

		m.offGroundTicks++
	}()

	// If the player is not jumping, we don't need to run this check.
	isJumping := utils.HasFlag(i.InputData, packet.InputFlagStartJumping)
	if !isJumping {
		return false
	}

	// Exempt if the player has recently teleported.
	if p.Teleporting() {
		return false
	}

	// If the player has been off ground for less than 10 ticks, don't check.
	if m.offGroundTicks <= 10 {
		return false
	}

	p.Flag(m, 1, map[string]any{})
	return false
}
