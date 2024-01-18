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
}

var defaultInputModes = map[protocol.DeviceOS]int{
	protocol.DeviceWin10:   packet.InputModeMouse,
	protocol.DeviceAndroid: packet.InputModeTouch,
	protocol.DeviceIOS:     packet.InputModeTouch,
}

func NewEditionFakerB() *EditionFakerB {
	d := &EditionFakerB{}
	d.Type = "EditionFaker"
	d.SubType = "B"

	d.Description = "Checks if the player's default input mode matches an expected value."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = 30 * player.TicksPerSecond

	d.FailBuffer = 1
	d.MaxBuffer = 1
	return d
}

func (d *EditionFakerB) ID() string {
	return DetectionIDEditionFakerB
}

func (d *EditionFakerB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	_, ok := pk.(*packet.TickSync)
	if !ok {
		return true
	}

	// Check that the default input mode of the client matches the expected input mode.
	if defaultInputMode, ok := defaultInputModes[p.ClientData().DeviceOS]; ok && defaultInputMode != p.ClientData().DefaultInputMode {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("title_id", utils.Device(p.ClientData().DeviceOS))
		data.Set("given_os", utils.InputMode(p.ClientData().DefaultInputMode))
		data.Set("expected_os", utils.InputMode(defaultInputMode))
		d.Fail(p, data)
		return false
	} else if !ok {
		p.Log().Warnf("unknown default input mode for device os %v (got %v)", p.ClientData().DeviceOS, p.ClientData().DefaultInputMode)
	}

	return true
}
