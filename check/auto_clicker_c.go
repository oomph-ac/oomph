package check

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerC checks for duplicated statistical values in clicks.
type AutoClickerC struct {
	samples []uint64
	basic
}

// NewAutoClickerC creates a new AutoClickerC check.
func NewAutoClickerC() *AutoClickerC {
	c := &AutoClickerC{}
	c.samples = make([]uint64, 0, 20)
	return c
}

// Name ...
func (*AutoClickerC) Name() (string, string) {
	return "AutoClicker", "C"
}

// Description ...
func (*AutoClickerC) Description() string {
	return "This checks for duplicated statistical values in clicks."
}

// MaxViolations ...
func (*AutoClickerC) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoClickerC) Process(p Processor, pk packet.Packet) bool {
	if !p.Clicking() {
		return false
	}
	a.samples = append(a.samples, p.ClickDelay())

	if len(a.samples) != 20 {
		return false
	}

	duplicates := game.Duplicates(a.samples)

	if (len(duplicates) > 4 && p.CPS() > 10) {
		p.Flag(a, a.violationAfterTicks(p.ClientTick(), 100), map[string]any{
			"Duplicates": len(duplicates),
			"CPS": p.CPS(),
		})
	}
	a.samples = a.samples[:0]
	
	return false
}
