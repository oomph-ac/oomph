package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDEditionFakerB = "oomph:edition_faker_b"

type EditionFakerB struct {
	BaseDetection
	run bool
}

var defaultInputModes = map[protocol.DeviceOS]int{
	protocol.DeviceWin10:   packet.InputModeMouse,
	protocol.DeviceAndroid: packet.InputModeTouch,
	protocol.DeviceIOS:     packet.InputModeTouch,
	protocol.DeviceFireOS:  packet.InputModeTouch,
	protocol.DeviceXBOX:    packet.InputModeGamePad,
	protocol.DeviceOrbis:   packet.InputModeGamePad,
	protocol.DeviceNX:      packet.InputModeGamePad,
}

func NewEditionFakerB() *EditionFakerB {
	d := &EditionFakerB{}
	d.Type = "EditionFaker"
	d.SubType = "B"

	d.Description = "Checks if the player's default input mode matches an expected value."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	d.run = true
	return d
}

func (d *EditionFakerB) ID() string {
	return DetectionIDEditionFakerB
}

func (d *EditionFakerB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if !d.run {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}
	d.run = false

	// Check that the default input mode of the client matches the expected input mode.
	if defaultInputMode, ok := defaultInputModes[p.ClientDat.DeviceOS]; ok && defaultInputMode != p.ClientDat.DefaultInputMode {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("defaultMode", utils.InputMode(p.ClientDat.DefaultInputMode))
		data.Set("expectedMode", utils.InputMode(defaultInputMode))
		d.Fail(p, data)
		return false
	}

	return true
}
