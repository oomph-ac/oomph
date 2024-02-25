package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDBadPacketA = "oomph:bad_packet_a"

type BadPacketA struct {
	BaseDetection
	prevFrame uint64
}

func NewBadPacketA() *BadPacketA {
	d := &BadPacketA{}
	d.Type = "BadPacket"
	d.SubType = "A"

	d.Description = "Checks if a player's simulation frame is valid."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *BadPacketA) ID() string {
	return DetectionIDBadPacketA
}

func (d *BadPacketA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	if i.Tick != (d.prevFrame+1) && d.prevFrame != 0 {
		p.Message("mismatch (%d, %d)", d, d.prevFrame, i.Tick)
	}

	diff := math32.Abs(float32(i.Tick) - float32(d.prevFrame))
	if d.prevFrame != 0 && diff > 2 {
		dat := orderedmap.NewOrderedMap[string, any]()
		dat.Set("curr", i.Tick)
		dat.Set("prev", d.prevFrame)
		d.Fail(p, dat)

		d.prevFrame = i.Tick
		return true
	}

	d.prevFrame = i.Tick
	return true
}
