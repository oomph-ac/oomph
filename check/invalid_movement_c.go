package check

import (
	"github.com/justtaldevelops/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// InvalidMovementC checks if the delay between the users jumps is invalid.
type InvalidMovementC struct {
	basic
	jumpTicks int32
}

// NewInvalidMovementC creates a new InvalidMovementC check.
func NewInvalidMovementC() *InvalidMovementC {
	return &InvalidMovementC{}
}

// Name ...
func (*InvalidMovementC) Name() (string, string) {
	return "InvalidMovement", "C"
}

// Description ...
func (*InvalidMovementC) Description() string {
	return "This checks if the delay between a users jumps is invalid. This can detect things such as air-jump or sometimes bunny hop."
}

// MaxViolations ...
func (*InvalidMovementC) MaxViolations() float64 {
	return 10
}

// Process ...
func (i *InvalidMovementC) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.PlayerAuthInput:
		i.jumpTicks--
		isHoldingJump := utils.HasFlag(pk.InputData, packet.InputFlagJumping)
		if !isHoldingJump {
			i.jumpTicks = 0
		}
		if processor.Jumping() {
			if i.jumpTicks > 0 || !isHoldingJump {
				processor.Flag(i, 1, map[string]interface{}{
					"Jumping Ticks": i.jumpTicks,
					"Jumping":       isHoldingJump,
				})
			}
			i.jumpTicks = 10
		}
	}
}
