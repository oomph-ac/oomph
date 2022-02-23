package check

import (
	"github.com/justtaldevelops/oomph/session"
	"github.com/justtaldevelops/oomph/settings"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoclickerB checks if the user is clicking above 18 cps with no double clicks.
type AutoclickerB struct {
	check
	samples []uint64
}

// Name ...
func (*AutoclickerB) Name() (string, string) {
	return "Autoclicker", "B"
}

// Description ...
func (*AutoclickerB) Description() string {
	return "This checks if the user is clicking above 18 cps with no double clicks."
}

// BaseSettings ...
func (*AutoclickerB) BaseSettings() settings.BaseSettings {
	return settings.Settings.AutoClicker.B
}

// Process ...
func (a *AutoclickerB) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		a.samples = append(a.samples, processor.Session().ClickDelay())
		if len(a.samples) == 20 {
			//processor.Debug(a, map[string]interface{}{"samples": a.samples})
			if func() bool {
				for _, v := range a.samples {
					if v == 0 {
						return false
					}
				}
				return true
			}() && processor.Session().CPS() >= 18 {
				processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 300), map[string]interface{}{"cps": processor.Session().CPS()})
			}
			a.samples = []uint64{}
		}
	}
}
