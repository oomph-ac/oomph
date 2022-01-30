package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoclickerD checks if a user has a constant and low standard deviation in their click data.
type AutoclickerD struct {
	check
	samples []uint64
}

// Name ...
func (*AutoclickerD) Name() (string, string) {
	return "Autoclicker", "D"
}

// Description ...
func (*AutoclickerD) Description() string {
	return "This checks if a user has a constant and low standard deviation in their click data."
}

// MaxViolations ...
func (*AutoclickerD) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*AutoclickerD) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (a *AutoclickerD) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		cps := processor.Session().CPS()
		var samples []float64
		for _, sample := range a.samples {
			samples = append(samples, float64(sample))
		}
		kurtosis, skewness, outliers, deviation := omath.Kurtosis(samples), omath.Skewness(samples), omath.Outliers(samples), omath.StandardDeviation(samples)
		processor.Debug(a, map[string]interface{}{"kurt": kurtosis, "skew": skewness, "outliers": outliers, "dev": deviation, "cps": cps})
		if kurtosis <= 0.05 && skewness < 0 && outliers == 0 && deviation <= 25 && cps >= 9 {
			if a.Buff(1, 2) > 1 {
				processor.Flag(a, map[string]interface{}{"kurtosis": omath.Round(kurtosis, 2), "skewness": omath.Round(skewness, 2), "cps": cps})
			}
		} else {
			a.Buff(-0.5)
		}
	}
}
