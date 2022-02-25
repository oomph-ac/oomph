package check

import (
	"github.com/justtaldevelops/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerC checks for an irregular clicking pattern using statistics.
type AutoClickerC struct {
	basic
	samples []float64
}

// NewAutoClickerC creates a new AutoClickerC check.
func NewAutoClickerC() *AutoClickerC {
	return &AutoClickerC{}
}

// Name ...
func (*AutoClickerC) Name() (string, string) {
	return "Autoclicker", "C"
}

// Description ...
func (*AutoClickerC) Description() string {
	return "This checks for an irregular clicking pattern."
}

// MaxViolations ...
func (*AutoClickerC) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoClickerC) Process(processor Processor, _ packet.Packet) {
	if processor.Clicking() {
		a.samples = append(a.samples, float64(processor.ClickDelay()))
		if len(a.samples) == 20 {
			cps := processor.CPS()
			deviation, skewness := game.StandardDeviation(a.samples), game.Skewness(a.samples)
			processor.Debug(a, map[string]interface{}{
				"Deviation": game.Round(deviation, 3),
				"Skewness":  game.Round(skewness, 3),
				"CPS":       cps,
			})
			if deviation <= 20 && (skewness > 1 || skewness == 0.0) && cps >= 9 {
				e := 5.0
				if skewness == 0.0 {
					e = 1.0
				}
				if a.Buff(1) >= e {
					processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 400), map[string]interface{}{
						"Deviation": game.Round(deviation, 3),
						"Skewness":  game.Round(skewness, 3),
						"CPS":       cps,
					})
				}
			} else {
				a.buffer = 0
			}
			a.samples = []float64{}
		}
	}
}
