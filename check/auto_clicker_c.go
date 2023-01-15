package check

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerC checks if a user has a constant and low standard deviation in their click data.
type AutoClickerC struct {
	basic
	samples []float64
}

// NewAutoClickerC creates a new AutoClickerC check.
func NewAutoClickerC() *AutoClickerC {
	c := &AutoClickerC{}
	c.samples = make([]float64, 0, 20)
	return c
}

// Name ...
func (*AutoClickerC) Name() (string, string) {
	return "AutoClicker", "C"
}

// Description ...
func (*AutoClickerC) Description() string {
	return "This checks if a user has a constant and low standard deviation in their click data."
}

// MaxViolations ...
func (*AutoClickerC) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoClickerC) Process(p Processor, _ packet.Packet) bool {
	if !p.Clicking() {
		return false
	}

	a.samples = append(a.samples, float64(p.ClickDelay()))
	if len(a.samples) != 20 {
		return false
	}

	cps := p.CPS()
	deviation, skewness := game.StandardDeviation(a.samples), game.Skewness(a.samples)
	p.Debug(a, map[string]any{
		"Deviation": game.Round64(deviation, 3),
		"Skewness":  game.Round64(skewness, 3),
		"CPS":       cps,
	})
	if deviation <= 20 && (skewness > 1 || skewness == 0.0) && cps >= 12 {
		e := 5.0
		if skewness == 0.0 {
			e = 1.0
		}
		if a.Buff(1) >= e {
			p.Flag(a, a.violationAfterTicks(p.ClientTick(), 400), map[string]any{
				"Deviation": game.Round64(deviation, 3),
				"Skewness":  game.Round64(skewness, 3),
				"CPS":       cps,
			})
		}
	} else {
		a.buffer = 0
	}
	a.samples = a.samples[:0]

	return false
}
