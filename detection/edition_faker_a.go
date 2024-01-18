package detection

import (
	"fmt"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/exp/slices"
)

const DetectionIDEditionFakerA = "oomph:edition_faker_a"

type EditionFakerA struct {
	BaseDetection
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

func NewEditionFakerA() *EditionFakerA {
	d := &EditionFakerA{}
	d.Type = "EditionFaker"
	d.SubType = "A"

	d.Description = "Checks if the player is faking their device OS."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = 30 * player.TicksPerSecond

	d.FailBuffer = 2
	d.MaxBuffer = 4
	return d
}

func (d *EditionFakerA) ID() string {
	return DetectionIDEditionFakerA
}

func (d *EditionFakerA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	_, ok := pk.(*packet.TickSync)
	if !ok {
		return true
	}

	deviceOS := p.ClientData().DeviceOS
		titleID := p.IdentityData().TitleID

		// Check if there's a titleID we know that is invalid/incompatiable with Minecraft: Bedrock Edition.
		if clientType, ok := invalidTitleIDs[titleID]; ok {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("title_id", titleID)
			data.Set("given_os", utils.Device(deviceOS))
			data.Set("expected_os", fmt.Sprintf("None (client %s should not support MC:BE)", clientType))
			d.Fail(p, data)
			return false
		}

		// 1904044383 is the title ID of the preview client in MC:BE. According to @GameParrot, the preview client
		// can be found on Windows, iOS, and Xbox.
		if titleID == "1904044383" && !slices.Contains(previewEditionClients, deviceOS) {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("title_id", titleID)
			data.Set("given_os", utils.Device(deviceOS))
			data.Set("expected_os", "Windows/iOS/Xbox")
			d.Fail(p, data)
			return false
		}

		// Check that the title ID matches the expected device OS.
		if expected, ok := knownTitleIDs[titleID]; ok && expected != deviceOS {
			if titleID == "2044456598" || titleID == "1828326430" {
				return false
			}
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("title_id", titleID)
			data.Set("given_os", utils.Device(deviceOS))
			data.Set("expected_os", utils.Device(expected))
			d.Fail(p, data)
		} else if !ok {
			p.Disconnect(fmt.Sprintf("report to admin: unknown title ID %s with OS %v", titleID, deviceOS))
			p.Log().Warnf("unknown title ID %s with OS %v", titleID, deviceOS)
		}


	return true
}
