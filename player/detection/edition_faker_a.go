package detection

import (
	"fmt"
	"slices"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

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

type EditionFakerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	run bool
}

func New_EditionFakerA(p *player.Player) *EditionFakerA {
	return &EditionFakerA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},

		run: true,
	}
}

func (*EditionFakerA) Type() string {
	return TYPE_EDITION_FAKER
}

func (*EditionFakerA) SubType() string {
	return "A"
}

func (*EditionFakerA) Description() string {
	return "Checks if the player is faking their device OS."
}

func (*EditionFakerA) Punishable() bool {
	return true
}

func (d *EditionFakerA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *EditionFakerA) Detect(pk packet.Packet) {
	if !d.run {
		return
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}
	d.run = false

	deviceOS := d.mPlayer.ClientDat.DeviceOS
	titleID := d.mPlayer.IdentityDat.TitleID

	// Check if there's a titleID we know that is invalid/incompatible with Minecraft: Bedrock Edition.
	if clientType, ok := invalidTitleIDs[titleID]; ok {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("titleID", titleID)
		data.Set("givenOS", utils.Device(deviceOS))
		data.Set("expectedOS", fmt.Sprintf("None (client %s should not support MC:BE)", clientType))
		d.mPlayer.FailDetection(d, data)
		return
	}

	// 1904044383 is the title ID of the preview client in MC:BE. According to @GameParrot, the preview client
	// can be found on Windows, iOS, and Xbox.
	if titleID == "1904044383" {
		if !slices.Contains(previewEditionClients, deviceOS) {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("titleID", titleID)
			data.Set("givenOS", utils.Device(deviceOS))
			data.Set("expectedOS", "Windows/iOS/Xbox")
			d.mPlayer.FailDetection(d, data)
		}
		return
	}

	// Check that the title ID matches the expected device OS.
	if expected, ok := knownTitleIDs[titleID]; ok && expected != deviceOS {
		// Exempt Playstation and XBOX devices from this check, since they need proxy workarounds to connect to external servers.
		if titleID == "2044456598" || titleID == "1828326430" {
			return
		}

		// Ugly & much [sugar honey iced tea] hack for BedrockTogether - why do console versions need external solutions to join servers anyway?
		if titleID == "1739947436" && (deviceOS == protocol.DeviceOrbis || deviceOS == protocol.DeviceXBOX) {
			return
		}

		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("titleID", titleID)
		data.Set("givenOS", utils.Device(deviceOS))
		data.Set("expectedOS", utils.Device(expected))
		d.mPlayer.FailDetection(d, data)
	} else if !ok {
		switch titleID {
		case "":
			if d.mPlayer.Version != player.GameVersion1_21_80 {
				d.mPlayer.Disconnect("TitleID not present")
				d.mPlayer.Log().Warn("no titleID present in identity data", "version", d.mPlayer.Version)
			}
		default:
			d.mPlayer.Disconnect(fmt.Sprintf("report to admin: unknown title ID %s with OS %v", titleID, deviceOS))
			d.mPlayer.Log().Warn("unknown title ID for given OS", "titleID", titleID, "deviceOS", deviceOS)
		}
	}
}
