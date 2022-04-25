package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerB checks if the user is clicking above 18 cps with no double clicks.
type AutoClickerB struct {
	samples []uint64
	basic
}

// NewAutoClickerB creates a new AutoClickerB check.
func NewAutoClickerB() *AutoClickerB {
	return &AutoClickerB{}
}

// Name ...
func (*AutoClickerB) Name() (string, string) {
	return "AutoClicker", "B"
}

// Description ...
func (*AutoClickerB) Description() string {
	return "This checks if the user is clicking above 18 cps with no double clicks."
}

// MaxViolations ...
func (*AutoClickerB) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoClickerB) Process(p Processor, _ packet.Packet) {
	if p.Clicking() {
		a.samples = append(a.samples, p.ClickDelay())
		if len(a.samples) == 20 {
			if a.verifySamples() && p.CPS() >= 18 {
				p.Flag(a, a.violationAfterTicks(p.ClientTick(), 300), map[string]interface{}{
					"CPS": p.CPS(),
				})
			}
			a.samples = make([]uint64, 0)
		}
	}
}

// verifySamples verifies the existing samples for any violations.
func (a *AutoClickerB) verifySamples() bool {
	for _, v := range a.samples {
		if v == 0 {
			return false
		}
	}
	return true
}
