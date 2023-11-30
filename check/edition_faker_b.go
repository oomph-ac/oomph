package check

import (
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type EditionFakerB struct {
	basic
}

func NewEditionFakerB() *EditionFakerB {
	return &EditionFakerB{}
}

func (*EditionFakerB) Name() (string, string) {
	return "EditionFaker", "B"
}

func (*EditionFakerB) Description() string {
	return "This checks if the player's default input mode matches an expected value."
}

func (*EditionFakerB) MaxViolations() float64 {
	return 1
}

var defaultInputModes = map[protocol.DeviceOS]int{
	protocol.DeviceWin10:   packet.InputModeMouse,
	protocol.DeviceAndroid: packet.InputModeTouch,
	protocol.DeviceIOS:     packet.InputModeTouch,
}

func (e *EditionFakerB) Process(p Processor, pk packet.Packet) bool {
	if _, ok := pk.(*packet.TickSync); !ok {
		return false
	}

	// Check that the default input mode of the client matches the expected input mode.
	if defaultInputMode, ok := defaultInputModes[p.ClientData().DeviceOS]; ok && defaultInputMode != p.ClientData().DefaultInputMode {
		p.Flag(e, 1, map[string]any{
			"OS":                          utils.Device(p.ClientData().DeviceOS),
			"Default Input Mode":          utils.InputMode(p.ClientData().DefaultInputMode),
			"Expected Default Input Mode": utils.InputMode(defaultInputMode),
		})
		return false
	} else if !ok {
		p.Log().Warnf("unknown default input mode  for device os %v (got %v)", p.ClientData().DeviceOS, p.ClientData().DefaultInputMode)
	}

	return false
}
