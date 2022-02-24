package check

import (
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerB checks if the user is clicking above 18 cps with no double clicks.
type AutoClickerB struct {
	check
	samples []uint64
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

// Process ...
func (a *AutoClickerB) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		a.samples = append(a.samples, processor.Session().ClickDelay())
		if len(a.samples) == 20 {
			if a.verifySamples() && processor.Session().CPS() >= 18 {
				processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 300), map[string]interface{}{
					"CPS": processor.Session().CPS(),
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
