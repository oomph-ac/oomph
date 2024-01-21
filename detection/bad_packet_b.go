package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDBadPacketB = "oomph:bad_packet_b"

type BadPacketB struct {
	BaseDetection
	last int
	tick int
}

func NewBadPacketB() *BadPacketB {
	d := &BadPacketB{}
	d.Type = "BadPacket"
	d.SubType = "B"

	d.Description = "Checks if a player is consistently sending MovePlayer packets rather than PlayerAuthInput packets."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *BadPacketB) ID() string {
	return DetectionIDBadPacketB
}

func (d *BadPacketB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	switch pk.(type) {
	case *packet.MovePlayer:
		s := d.tick - d.last
		if s < 2 && !p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler).Immobile {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("speed", s)
			d.Fail(p, data)
			return false
		}
	case *packet.PlayerAuthInput:
		d.tick++
	}

	return true
}
