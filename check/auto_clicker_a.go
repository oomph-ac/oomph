package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerA checks if the player cps is over a certain threshold.
type AutoClickerA struct {
	basic
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

// MaxViolations ...
func (*AutoClickerA) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoClickerA) Process(p Processor, _ packet.Packet) bool {
	if p.Clicking() && p.CPS() > 22 {
		p.Flag(a, a.violationAfterTicks(p.ClientTick(), 40), map[string]any{
			"CPS": p.CPS(),
		})
	}

	return false
}
