package check

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerD checks for an irregular clicking pattern using statistics.
type AutoClickerD struct {
	samples []float64
	basic
}

// NewAutoClickerD creates a new AutoClickerD check.
func NewAutoClickerD() *AutoClickerD {
	c := &AutoClickerD{}
	c.samples = make([]float64, 0, 20)
	return c
}

// Name ...
func (*AutoClickerD) Name() (string, string) {
	return "AutoClicker", "D"
}

// Description ...
func (*AutoClickerD) Description() string {
	return "This checks for an irregular clicking pattern."
}

// MaxViolations ...
func (*AutoClickerD) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoClickerD) Process(p Processor, _ packet.Packet) bool {
	if !p.Clicking() {
		return false
	}

	a.samples = append(a.samples, float64(p.ClickDelay()))
	if len(a.samples) != 20 {
		// Not enough samples, wait until we have more.
		return false
	}

	cps := p.CPS()
	kurtosis, skewness, outliers, deviation := game.Kurtosis(a.samples), game.Skewness(a.samples), game.Outliers(a.samples), game.StandardDeviation(a.samples)
	p.Debug(a, map[string]any{
		"Kurtosis":  game.Round64(kurtosis, 2),
		"Skewness":  game.Round64(skewness, 2),
		"Outliers":  outliers,
		"Deviation": deviation,
		"CPS":       cps,
	})
	if kurtosis <= 0.05 && skewness < 0 && outliers == 0 && deviation <= 25 && cps >= 9 {
		if a.Buff(1, 2) > 1 {
			p.Flag(a, a.violationAfterTicks(p.ClientTick(), 400), map[string]any{
				"Kurtosis": game.Round64(kurtosis, 2),
				"Skewness": game.Round64(skewness, 2),
				"CPS":      cps,
			})
		}
	} else {
		a.Buff(-0.5)
	}
	a.samples = a.samples[:0]

	return false
}
