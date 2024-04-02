package detection

import (
	"fmt"
	"math"

	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDAutoClickerC = "oomph:auto_clicker_c"

type AutoClickerC struct {
	clickSamples []float64
	statSamples  []string
	BaseDetection
}

func NewAutoClickerC() *AutoClickerC {
	d := &AutoClickerC{}
	d.Type = "AutoClicker"
	d.SubType = "C"

	d.Description = "Checks for duplicated statistical values in clicks."
	d.Punishable = false

	d.MaxViolations = math.MaxFloat32
	d.trustDuration = 20 * player.TicksPerSecond

	d.FailBuffer = 2
	d.MaxBuffer = 10
	return d
}

func (d *AutoClickerC) ID() string {
	return DetectionIDAutoClickerC
}

func (d *AutoClickerC) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	c := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)
	if !c.Clicking {
		return true
	}
	d.clickSamples = append(d.clickSamples, float64(c.ClickDelay))
	if len(d.clickSamples) != 20 {
		return true
	}

	interpolatedCPS := 20 / math32.Min(0.05, float32(game.Mean(d.clickSamples)))
	if interpolatedCPS < 10 {
		d.clickSamples = make([]float64, 0, 20)
		return true
	}

	d.statSamples = append(d.statSamples, fmt.Sprintf("%v %v %v", game.Kurtosis(d.clickSamples), game.Skewness(d.clickSamples), float64(game.Outliers(d.clickSamples))))
	d.clickSamples = make([]float64, 0, 20)

	if len(d.statSamples) != 7 {
		return true
	}

	dupes := d.duplicates()
	d.statSamples = d.statSamples[1:]

	if dupes >= 4 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("duplicates", dupes)
		data.Set("cps", c.CPS)
		d.Fail(p, data)
		d.statSamples = make([]string, 0, 10)
	}

	return true
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
