package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoclickerB checks if the user is clicking above 18 cps with no double clicks.
type AutoclickerB struct {
	basic
	samples []uint64
}

// NewAutoClickerB creates a new AutoclickerB check.
func NewAutoClickerB() *AutoclickerB {
	return &AutoclickerB{}
}

// Name ...
func (*AutoclickerB) Name() (string, string) {
	return "Autoclicker", "B"
}

// Description ...
func (*AutoclickerB) Description() string {
	return "This checks if the user is clicking above 18 cps with no double clicks."
}

// MaxViolations ...
func (*AutoclickerB) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoclickerB) Process(processor Processor, _ packet.Packet) {
	if processor.Clicking() {
		a.samples = append(a.samples, processor.ClickDelay())
		if len(a.samples) == 20 {
			if a.verifySamples() && processor.CPS() >= 18 {
				processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 300), map[string]interface{}{
					"CPS": processor.CPS(),
				})
			}
			a.samples = make([]uint64, 0)
		}
	}
}

// verifySamples verifies the existing samples for any violations.
func (a *AutoclickerB) verifySamples() bool {
	for _, v := range a.samples {
		if v == 0 {
			return false
		}
	}
	return true
}
