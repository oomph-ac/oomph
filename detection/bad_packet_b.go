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
		speed := d.tick - d.last
		mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
		cDat := p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler)

		if speed < 2 && !mDat.Immobile && cDat.InLoadedChunk {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("speed", speed)
			d.Fail(p, data)
			return false
		}
	case *packet.PlayerAuthInput:
		d.tick++
	}

	return true
}
