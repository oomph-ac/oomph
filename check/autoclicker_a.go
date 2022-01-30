package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// AutoclickerA checks if the player cps is over a certain threshold.
type AutoclickerA struct {
	check
}

// Name ...
func (*AutoclickerA) Name() (string, string) {
	return "Autoclicker", "A"
}

// Description ...
func (*AutoclickerA) Description() string {
	return "This checks if a players cps is over a certain threshold."
}

// MaxViolations ...
func (*AutoclickerA) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*AutoclickerA) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (a *AutoclickerA) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		if processor.Session().CPS() > 22 {
			processor.Flag(a, map[string]interface{}{"cps": processor.Session().CPS()})
		} else {
			a.violations = math.Max(a.violations-0.0075, 0)
		}
	}
}
