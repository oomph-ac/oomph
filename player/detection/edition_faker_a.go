package detection

import (
	"fmt"
	"slices"

	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	titleIDAndroid = "1739947436"
	titleIDIOS     = "1810924247"
	titleIDFireOS  = "1944307183"
	titleIDWindows = "896928775"
	titleIDOrbis   = "2044456598"
	titleIDNX      = "2047319603"
	titleIDXBOX    = "1828326430"
	titleIDWP      = "1916611344"
	titleIDPreview = "1904044383"
)

var knownTitleIDs = map[protocol.DeviceOS]string{
	protocol.DeviceAndroid: titleIDAndroid,
	protocol.DeviceIOS:     titleIDIOS,
	protocol.DeviceFireOS:  titleIDFireOS,

	protocol.DeviceWin10: titleIDWindows, // Windows (UWP) on MC:BE Versions 1.21.111 and below
	protocol.DeviceWin32: titleIDWindows, // Windows (GDK) on MC:BE Versions 1.21.120 and above

	protocol.DeviceOrbis: titleIDOrbis,
	protocol.DeviceNX:    titleIDNX,
	protocol.DeviceXBOX:  titleIDXBOX,
	14:                   titleIDWP, // protocol.DeviceWP
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
	return TypeEditionFaker
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

	// 1904044383 is the title ID of the preview client in MC:BE. According to @GameParrot, the preview client
	// can be found on Windows, iOS, and Xbox.
	if titleID == titleIDPreview {
		if !slices.Contains(previewEditionClients, deviceOS) {
			d.mPlayer.FailDetection(
				d,
				"titleID", titleID,
				"givenOS", utils.Device(deviceOS),
				"expectedOS", "Windows/iOS/Xbox",
			)
		}
		return
	}

	// Check if the client is trying to log in with a GDK client where GDK is not available.
	if deviceOS == protocol.DeviceWin32 && d.mPlayer.Version < player.GameVersion1_21_120 {
		d.mPlayer.FailDetection(
			d,
			"titleID", titleID,
			"givenOS", utils.Device(deviceOS),
			"expectedOS", "Windows (UWP)",
			"protocol", d.mPlayer.Version,
		)
		return
	}

	// Check that the title ID matches the expected device OS.
	if expected, ok := knownTitleIDs[deviceOS]; ok && expected != titleID {
		// TODO: Is this still required? Most of these utilities rely on just sending transfer packets to those devices.
		if titleID == titleIDOrbis || titleID == titleIDXBOX {
			return
		}

		// Ugly & much [sugar honey iced tea] hack for BedrockTogether - why do console versions need external solutions to join servers anyway?
		if titleID == titleIDAndroid && (deviceOS == protocol.DeviceOrbis || deviceOS == protocol.DeviceXBOX) {
			return
		}

		// Bug with old game version
		if len(titleID) == 0 && d.mPlayer.Version == player.GameVersion1_21_80 {
			return
		}

		d.mPlayer.FailDetection(
			d,
			"titleID", titleID,
			"givenOS", utils.Device(deviceOS),
			"expectedTitleID", expected,
		)
	} else if !ok {
		d.mPlayer.Disconnect(fmt.Sprintf("report to admin: unknown title ID %s with OS %v", titleID, deviceOS))
		d.mPlayer.Log().Warn("unknown title ID for given OS", "titleID", titleID, "deviceOS", deviceOS)
	}
}
