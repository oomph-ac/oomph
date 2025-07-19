package detection

import (
	"slices"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var knownInvalidInputs = map[protocol.DeviceOS][]uint32{
	protocol.DeviceOrbis: {packet.InputModeTouch}, // Playstation
	protocol.DeviceXBOX:  {packet.InputModeTouch},
}

type EditionFakerC struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_EditionFakerC(p *player.Player) *EditionFakerC {
	return &EditionFakerC{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 1,
		},
	}
}

func (*EditionFakerC) Type() string {
	return TypeEditionFaker
}

func (*EditionFakerC) SubType() string {
	return "C"
}

func (*EditionFakerC) Description() string {
	return "Checks if the player has an invalid input mode for their given device."
}

func (*EditionFakerC) Punishable() bool {
	return true
}

func (d *EditionFakerC) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *EditionFakerC) Detect(pk packet.Packet) {
	if i, ok := pk.(*packet.PlayerAuthInput); ok {
		// There is no input mode after motion controller or before mouse.
		if i.InputMode > packet.InputModeMotionController || i.InputMode < packet.InputModeMouse {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("inputMode", i.InputMode)
			d.mPlayer.FailDetection(d, data)
			return
		}

		if invalid, ok := knownInvalidInputs[d.mPlayer.ClientDat.DeviceOS]; !ok && slices.Contains(invalid, i.InputMode) {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("inputMode", i.InputMode)
			data.Set("OS", utils.Device(d.mPlayer.ClientDat.DeviceOS))
		}
	}
}
