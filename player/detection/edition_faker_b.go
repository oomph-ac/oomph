package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var defaultInputModes = map[protocol.DeviceOS]int{
	protocol.DeviceWin10:   packet.InputModeMouse,
	protocol.DeviceWin32:   packet.InputModeMouse,
	protocol.DeviceAndroid: packet.InputModeTouch,
	protocol.DeviceIOS:     packet.InputModeTouch,
	protocol.DeviceFireOS:  packet.InputModeTouch,
	protocol.DeviceXBOX:    packet.InputModeGamePad,
	protocol.DeviceOrbis:   packet.InputModeGamePad,
	protocol.DeviceNX:      packet.InputModeGamePad,
}

type EditionFakerB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	run bool
}

func New_EditionFakerB(p *player.Player) *EditionFakerB {
	return &EditionFakerB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 1,
		},

		run: true,
	}
}

func (*EditionFakerB) Type() string {
	return TypeEditionFaker
}

func (*EditionFakerB) SubType() string {
	return "B"
}

func (*EditionFakerB) Description() string {
	return "Checks if the player's default input mode matches an expected value."
}

func (*EditionFakerB) Punishable() bool {
	return true
}

func (d *EditionFakerB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *EditionFakerB) Detect(pk packet.Packet) {
	if !d.run {
		return
	}

	if _, ok := pk.(*packet.PlayerAuthInput); ok {
		d.run = false

		// Check that the default input mode of the client matches the expected input mode.
		currentDefaultInputMode := d.mPlayer.ClientDat.DefaultInputMode
		if defaultInputMode, ok := defaultInputModes[d.mPlayer.ClientDat.DeviceOS]; ok && defaultInputMode != currentDefaultInputMode {
			d.mPlayer.FailDetection(
				d,
				"defaultMode", utils.InputMode(currentDefaultInputMode),
				"expectedMode", utils.InputMode(defaultInputMode),
			)
		}
	}
}
