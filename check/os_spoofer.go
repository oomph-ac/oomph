package check

import (
	"github.com/justtaldevelops/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// OSSpoofer checks if the player's device os does not equal the one that matches with their title id.
type OSSpoofer struct {
	check
	// TODO: Make this better.
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

// Process ...
func (o *OSSpoofer) Process(processor Processor, _ packet.Packet) {
	if expected, ok := map[string]protocol.DeviceOS{
		"1739947436": protocol.DeviceAndroid,
		"1810924247": protocol.DeviceIOS,
		"1944307183": protocol.DeviceFireOS,
		"896928775":  protocol.DeviceWin10,
		"2044456598": protocol.DeviceOrbis,
		"2047319603": protocol.DeviceNX,
		"1828326430": protocol.DeviceXBOX,
		"1916611344": protocol.DeviceWP,
		// TODO: Add more title IDs.
	}[o.TitleID]; ok && expected != o.GivenOS {
		processor.Flag(o, 1, map[string]interface{}{
			"Title ID":    o.TitleID,
			"Given OS":    utils.Device(o.GivenOS),
			"Expected OS": utils.Device(expected),
		})
	} else if !ok {
		processor.Debug(o, map[string]interface{}{
			"Unknown Title ID": o.TitleID,
			"Given OS":         utils.Device(o.GivenOS),
		})
	}
}
