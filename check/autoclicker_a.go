package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoclickerA checks if the player cps is over a certain threshold.
type AutoclickerA struct {
	basic
}

// NewAutoClickerA creates a new AutoclickerA check.
func NewAutoClickerA() *AutoclickerA {
	return &AutoclickerA{}
}

// Name ...
func (*AutoclickerA) Name() (string, string) {
	return "Autoclicker", "A"
}

// Description ...
func (*AutoclickerA) Description() string {
	return "This checks if a players cps is over a certain threshold."
}

// MaxViolations ...
func (*AutoclickerA) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoclickerA) Process(processor Processor, _ packet.Packet) {
	if processor.Clicking() && processor.CPS() > 22 {
		processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 40), map[string]interface{}{
			"CPS": processor.CPS(),
		})
	}
}
