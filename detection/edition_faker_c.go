package detection

import (
	"slices"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDEditionFakerC = "oomph:edition_faker_c"

var knownInvalidInputs = map[protocol.DeviceOS][]uint32{
	protocol.DeviceOrbis: {packet.InputModeTouch},
	protocol.DeviceXBOX:  {packet.InputModeTouch},
}

type EditionFakerC struct {
	BaseDetection
}

func NewEditionFakerC() *EditionFakerC {
	d := &EditionFakerC{}
	d.Type = "EditionFaker"
	d.SubType = "C"

	d.Description = "Checks if the player has an invalid input mode for their given device."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *EditionFakerC) ID() string {
	return DetectionIDEditionFakerC
}

func (d *EditionFakerC) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if i, ok := pk.(*packet.PlayerAuthInput); ok {
		// There is no input mode after motion controller or before mouse.
		if i.InputMode > packet.InputModeMotionController || i.InputMode < packet.InputModeMouse {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("inputMode", i.InputMode)
			d.Fail(p, data)
			return true
		}

		invalid, ok := knownInvalidInputs[p.ClientDat.DeviceOS]
		if !ok {
			return true
		}

		if slices.Contains(invalid, i.InputMode) {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("inputMode", i.InputMode)
			data.Set("OS", utils.Device(p.ClientDat.DeviceOS))
		}
	}

	return true
}
