package check

import (
	"github.com/justtaldevelops/oomph/session"
	"github.com/justtaldevelops/oomph/settings"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

// BaseSettings ...
func (*AutoclickerA) BaseSettings() settings.BaseSettings {
	return settings.Settings.AutoClicker.A.BaseSettings
}

// Process ...
func (a *AutoclickerA) Process(processor Processor, _ packet.Packet) {
	if processor.Session().HasFlag(session.FlagClicking) {
		if processor.Session().CPS() > 22 {
			processor.Flag(a, a.updateAndGetViolationAfterTicks(processor.ClientTick(), 40), map[string]interface{}{"cps": processor.Session().CPS()})
		}
	}
}
