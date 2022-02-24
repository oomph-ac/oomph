package check

import (
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerC checks for an irregular clicking pattern using statistics.
type AutoClickerC struct {
	check
	samples []float64
}

// NewAutoClickerC creates a new AutoClickerC check.
func NewAutoClickerC() *AutoClickerC {
	return &AutoClickerC{}
}

// Name ...
func (*AutoClickerC) Name() (string, string) {
	return "AutoClicker", "C"
}

// Description ...
func (*AutoClickerC) Description() string {
	return "This checks for an irregular clicking pattern."
}

// Process ...
func (a *AutoClickerC) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		a.samples = append(a.samples, float64(processor.Session().ClickDelay()))
		if len(a.samples) == 20 {
			cps := processor.Session().CPS()
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
