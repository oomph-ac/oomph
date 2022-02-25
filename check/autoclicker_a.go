package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerA checks if the player cps is over a certain threshold.
type AutoClickerA struct {
	check
}

// NewAutoClickerA creates a new AutoClickerA check.
func NewAutoClickerA() *AutoClickerA {
	return &AutoClickerA{}
}

// Name ...
func (*AutoClickerA) Name() (string, string) {
	return "AutoClicker", "A"
}

// Description ...
func (*AutoClickerA) Description() string {
	return "This checks if a players cps is over a certain threshold."
}

// Process ...
func (a *AutoClickerA) Process(processor Processor, _ packet.Packet) {
	if processor.Clicking() && processor.CPS() > 22 {
		processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 40), map[string]interface{}{
			"CPS": processor.CPS(),
		})
	}
}
