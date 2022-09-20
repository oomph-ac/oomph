package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/exp/slices"
)

// AutoClickerB checks if the user is clicking above 18 cps with no double clicks.
type AutoClickerB struct {
	samples []uint64
	basic
}

// NewAutoClickerB creates a new AutoClickerB check.
func NewAutoClickerB() *AutoClickerB {
	c := &AutoClickerB{}
	c.samples = make([]uint64, 0, 20)
	return c
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
func (a *AutoClickerB) Process(p Processor, _ packet.Packet) bool {
	if !p.Clicking() {
		return false
	}

	a.samples = append(a.samples, p.ClickDelay())
	if len(a.samples) != 20 {
		return false
	}

	if !slices.Contains(a.samples, 0) && p.CPS() >= 18 {
		p.Flag(a, a.violationAfterTicks(p.ClientTick(), 300), map[string]any{
			"CPS": p.CPS(),
		})
	}
	a.samples = a.samples[:0]

	return false
}
