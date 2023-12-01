package check

import (
	"fmt"

	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoClickerC checks for duplicated statistical values in clicks.
type AutoClickerC struct {
	clickSamples []float64
	statSamples  []string

	basic
}

// NewAutoClickerC creates a new AutoClickerC check.
func NewAutoClickerC() *AutoClickerC {
	c := &AutoClickerC{}
	c.clickSamples = make([]float64, 0, 20)
	c.statSamples = make([]string, 0, 10)

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

	a.clickSamples = append(a.clickSamples, float64(p.ClickDelay()))
	if len(a.clickSamples) != 20 {
		return false
	}

	interpolatedCPS := 20 / math32.Min(0.05, float32(game.Mean(a.clickSamples)))
	if interpolatedCPS < 10 {
		a.clickSamples = make([]float64, 0, 20)
		return false
	}

	a.statSamples = append(a.statSamples, fmt.Sprintf("%v %v %v", game.Kurtosis(a.clickSamples), game.Skewness(a.clickSamples), float64(game.Outliers(a.clickSamples))))
	a.clickSamples = make([]float64, 0, 20)

	if len(a.statSamples) != 7 {
		return false
	}

	dupes := a.duplicates()
	a.statSamples = a.statSamples[1:]
	data := map[string]any{
		"duplicates": dupes,
		"cps":        p.CPS(),
	}

	if dupes >= 4 {
		p.Flag(a, 1, data)
		a.statSamples = make([]string, 0, 10)
	}

	p.Debug(a, data)
	return false
}

func (a *AutoClickerC) duplicates() int {
	count := 0
	for i, sample1 := range a.statSamples {
		for j, sample2 := range a.statSamples {
			if i == j {
				continue
			}

			if sample1 == sample2 {
				count++
			}
		}
	}

	return count
}
