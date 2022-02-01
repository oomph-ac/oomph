package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AutoclickerB checks if the user is clicking above 16 cps with no double clicks.
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
	return "This checks if the user is clicking above 16 cps with no double clicks."
}

// MaxViolations ...
func (*AutoclickerB) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*AutoclickerB) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (a *AutoclickerB) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		a.samples = append(a.samples, processor.Session().ClickDelay())
		if len(a.samples) == 20 {
			processor.Debug(a, map[string]interface{}{"samples": a.samples})
			if func() bool {
				for _, v := range a.samples {
					if v == 0 {
						return false
					}
				}
				return true
			}() && processor.Session().CPS() >= 18 {
				processor.Flag(a, map[string]interface{}{"cps": processor.Session().CPS()})
			} else {
				a.Buff(-0.025)
			}
			a.samples = []uint64{}
		}
	}
}
