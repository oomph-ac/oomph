package check

import (
	"github.com/justtaldevelops/oomph/minecraft"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerD checks if a user has a constant and low standard deviation in their click data.
type AutoClickerD struct {
	check
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

// Process ...
func (a *AutoClickerD) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		a.samples = append(a.samples, float64(processor.Session().ClickDelay()))
		if len(a.samples) < 20 {
			// Not enough samples, wait until we have more.
			return
		}

		cps := processor.Session().CPS()
		kurtosis, skewness, outliers, deviation := minecraft.Kurtosis(a.samples), minecraft.Skewness(a.samples), minecraft.Outliers(a.samples), minecraft.StandardDeviation(a.samples)
		processor.Debug(a, map[string]interface{}{
			"Kurtosis":  minecraft.Round(kurtosis, 2),
			"Skewness":  minecraft.Round(skewness, 2),
			"Outliers":  outliers,
			"Deviation": deviation,
			"CPS":       cps,
		})
		if kurtosis <= 0.05 && skewness < 0 && outliers == 0 && deviation <= 25 && cps >= 9 {
			if a.Buff(1, 2) > 1 {
				processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 400), map[string]interface{}{
					"Kurtosis": minecraft.Round(kurtosis, 2),
					"Skewness": minecraft.Round(skewness, 2),
					"CPS":      cps,
				})
			}
		} else {
			a.Buff(-0.5)
		}
		a.samples = []float64{}
	}
}
