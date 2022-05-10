package check

import (
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// OSSpoofer checks if the player's device os does not equal the one that matches with their title id.
type OSSpoofer struct {
	basic
}

// NewOSSpoofer creates a new OSSpoofer check.
func NewOSSpoofer() *OSSpoofer {
	return &OSSpoofer{}
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
func (*OSSpoofer) MaxViolations() float64 {
	return 1
}

// Process ...
func (o *OSSpoofer) Process(p Processor, pk packet.Packet) {
	switch pk.(type) {
	case *packet.TickSync: // Sent by the client right as it spawns in.
		deviceOS := p.ClientData().DeviceOS
		if deviceOS == protocol.DeviceXBOX || deviceOS == protocol.DeviceOrbis {
			// Console players have to use a proxy to join servers, which would change their device os.
			return
		}

		titleID := p.IdentityData().TitleID
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
		}[titleID]; ok && expected != deviceOS {
			p.Flag(o, 1, map[string]any{
				"Title ID":    titleID,
				"Given OS":    utils.Device(deviceOS),
				"Expected OS": utils.Device(expected),
			})
		} else if !ok {
			p.Debug(o, map[string]any{
				"Unknown Title ID": titleID,
				"Given OS":         utils.Device(deviceOS),
			})
		}
	}
}
