package check

import (
	"fmt"
	"slices"

	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type EditionFakerA struct {
	basic
}

func NewEditionFakerA() *EditionFakerA {
	return &EditionFakerA{}
}

func (*EditionFakerA) Name() (string, string) {
	return "EditionFaker", "A"
}

func (*EditionFakerA) Description() string {
	return "This checks if the player is faking their device os."
}

func (*EditionFakerA) MaxViolations() float64 {
	return 1
}

var knownTitleIDs = map[string]protocol.DeviceOS{
	"1739947436": protocol.DeviceAndroid,
	"1810924247": protocol.DeviceIOS,
	"1944307183": protocol.DeviceFireOS,
	"896928775":  protocol.DeviceWin10,
	"2044456598": protocol.DeviceOrbis,
	"2047319603": protocol.DeviceNX,
	"1828326430": protocol.DeviceXBOX,
	"1916611344": protocol.DeviceWP,
}

var invalidTitleIDs = map[string]string{
	"328178078": "XBOX Mobile App",
}

var previewEditionClients = []protocol.DeviceOS{
	protocol.DeviceWin10,
	protocol.DeviceIOS,
	protocol.DeviceXBOX,
}

// Process ...
func (o *EditionFakerA) Process(p Processor, pk packet.Packet) bool {
	switch pk.(type) {
	case *packet.TickSync: // Sent by the client right as it spawns in.
		deviceOS := p.ClientData().DeviceOS
		titleID := p.IdentityData().TitleID

		// Check if there's a titleID we know that is invalid/incompatiable with Minecraft: Bedrock Edition.
		if clientType, ok := invalidTitleIDs[titleID]; ok {
			p.Flag(o, 1, map[string]any{
				"Title ID":    titleID,
				"Given OS":    utils.Device(deviceOS),
				"Expected OS": fmt.Sprintf("None (client %s should not support MC:BE)", clientType),
			})
			return false
		}

		// 1904044383 is the title ID of the preview client in MC:BE. According to @GameParrot, the preview client
		// can be found on Windows, iOS, and Xbox.
		if titleID == "1904044383" && !slices.Contains(previewEditionClients, deviceOS) {
			p.Flag(o, 1, map[string]any{
				"Title ID":    titleID,
				"Given OS":    utils.Device(deviceOS),
				"Expected OS": "Windows/iOS/Xbox",
			})
			return false
		}

		// Check that the title ID matches the expected device OS.
		if expected, ok := knownTitleIDs[titleID]; ok && expected != deviceOS {
			if titleID == "2044456598" || titleID == "1828326430" {
				// rawr XD! prowxy wockys made some fwucky wuckys in their code! now we have to ignore console players
				return false
			}
			p.Flag(o, 1, map[string]any{
				"Title ID":    titleID,
				"Given OS":    utils.Device(deviceOS),
				"Expected OS": utils.Device(expected),
			})
		} else if !ok {
			p.Disconnect(fmt.Sprintf("report to admin: unknown title ID %s with OS %v", titleID, deviceOS))
			p.Log().Warnf("unknown title ID %s with OS %v", titleID, deviceOS)
		}
	}

	return false
}
