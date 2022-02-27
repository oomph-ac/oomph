package check

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerD checks if a user has a constant and low standard deviation in their click data.
type AutoClickerD struct {
	basic
	samples []float64
}

// NewAutoClickerD creates a new AutoClickerD check.
func NewAutoClickerD() *AutoClickerD {
	return &AutoClickerD{}
}

// Name ...
func (*AutoClickerD) Name() (string, string) {
	return "AutoClicker", "D"
}

// Description ...
func (*AutoClickerD) Description() string {
	return "This checks if a user has a constant and low standard deviation in their click data."
}

// MaxViolations ...
func (*AutoClickerD) MaxViolations() float64 {
	return 15
}

// Process ...
func (a *AutoClickerD) Process(processor Processor, _ packet.Packet) {
	if processor.Clicking() {
		a.samples = append(a.samples, float64(processor.ClickDelay()))
		if len(a.samples) < 20 {
			// Not enough samples, wait until we have more.
			return
		}

		cps := processor.CPS()
		kurtosis, skewness, outliers, deviation := game.Kurtosis(a.samples), game.Skewness(a.samples), game.Outliers(a.samples), game.StandardDeviation(a.samples)
		processor.Debug(a, map[string]interface{}{
			"Kurtosis":  game.Round(kurtosis, 2),
			"Skewness":  game.Round(skewness, 2),
			"Outliers":  outliers,
			"Deviation": deviation,
			"CPS":       cps,
		})
		if kurtosis <= 0.05 && skewness < 0 && outliers == 0 && deviation <= 25 && cps >= 9 {
			if a.Buff(1, 2) > 1 {
				processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 400), map[string]interface{}{
					"Kurtosis": game.Round(kurtosis, 2),
					"Skewness": game.Round(skewness, 2),
					"CPS":      cps,
				})
			}
		} else {
			a.Buff(-0.5)
		}
		a.samples = []float64{}
	}
}
