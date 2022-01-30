package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoclickerC checks for an irregular clicking pattern using statistics.
type AutoclickerC struct {
	check
	samples []float64
}

// Name ...
func (*AutoclickerC) Name() (string, string) {
	return "Autoclicker", "C"
}

// Description ...
func (*AutoclickerC) Description() string {
	return "This checks for an irregular clicking pattern."
}

// MaxViolations ...
func (*AutoclickerC) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*AutoclickerC) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (a *AutoclickerC) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		a.samples = append(a.samples, float64(processor.Session().ClickDelay()))
		if len(a.samples) == 20 {
			cps := processor.Session().CPS()
			deviation, skewness := omath.StandardDeviation(a.samples), omath.Skewness(a.samples)
			processor.Debug(a, map[string]interface{}{"deviation": deviation, "skewness": skewness, "cps": cps})
			if deviation <= 20 && (skewness > 1 || skewness == 0.0) && cps >= 9 {
				var e float64
				if skewness == 0.0 {
					e = 1
				} else {
					e = 5
				}
				if a.Buff(1) >= e {
					processor.Flag(a, map[string]interface{}{"cps": cps, "dv": omath.Round(deviation, 3), "sk": omath.Round(skewness, 3)})
				}
			} else {
				a.buffer = 0
			}
			a.samples = []float64{}
		}
	}
}
