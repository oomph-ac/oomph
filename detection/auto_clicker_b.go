package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/exp/slices"
)

const DetectionIDAutoClickerB = "oomph:auto_clicker_b"

type AutoClickerB struct {
	samples []uint64
	BaseDetection
}

func NewAutoClickerB() *AutoClickerB {
	d := &AutoClickerB{}
	d.Type = "AutoClicker"
	d.SubType = "B"

	d.Description = "Checks if a player is clicking above 18 cps with no double clicks."
	d.Punishable = true

	d.MaxViolations = 20
	d.trustDuration = 20 * player.TicksPerSecond

	d.FailBuffer = 2
	d.MaxBuffer = 10
	return d
}

func (d *AutoClickerB) ID() string {
	return DetectionIDAutoClickerB
}

func (d *AutoClickerB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	c := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)
	if !c.Clicking {
		return true
	}
	d.samples = append(d.samples, uint64(c.ClickDelay))
	if len(d.samples) != 20 {
		return true
	}
	if !slices.Contains(d.samples, 0) && c.CPS >= 18 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("cps", c.CPS)
		d.Fail(p, data)
		return false
	}
	d.samples = d.samples[:0]
	return true
}
