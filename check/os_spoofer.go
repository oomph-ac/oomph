package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/oomph"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// OSSpoofer checks if the player's device os does not equal the one that matches with their title id.
type OSSpoofer struct {
	check
	GivenOS protocol.DeviceOS
	TitleID string
}

// Name ...
func (*OSSpoofer) Name() (string, string) {
	return "OS Spoofer", "A"
}

// Description ...
func (*OSSpoofer) Description() string {
	return "This checks if the player is faking their device os."
}

// MaxViolations ...
func (*OSSpoofer) MaxViolations() uint32 {
	return 1
}

// Punishment ...
func (*OSSpoofer) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (o *OSSpoofer) Process(processor Processor, _ packet.Packet) {
	givenString := "Unknown"
	if given, ok := oomph.DeviceOSToString[o.GivenOS]; ok {
		givenString = given
	}
	if expected, ok := oomph.TitleIds[o.TitleID]; ok && expected != o.GivenOS {
		expectedString := "Unknown"
		if exp, ok := oomph.DeviceOSToString[expected]; ok {
			expectedString = exp
		}
		processor.Flag(o, map[string]interface{}{"Title ID": o.TitleID, "Given OS": givenString, "Expected OS": expectedString})
	} else if !ok {
		processor.Debug(o, map[string]interface{}{"Unknown Title ID": o.TitleID, "Given OS": givenString})
	}
}
