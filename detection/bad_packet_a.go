package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDBadPacketA = "oomph:bad_packet_a"

type BadPacketA struct {
	BaseDetection
}

func NewBadPacketA() *BadPacketA {
	d := &BadPacketA{}
	d.Type = "BadPacket"
	d.SubType = "A"

	d.Description = "Checks if a player's simulation frame is valid."
	d.Punishable = true

	d.MaxViolations = 5
	d.trustDuration = 30 * player.TicksPerSecond

	d.FailBuffer = 2
	d.MaxBuffer = 4
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

	c := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)
	if c.LastFrame != 0 && i.Tick == 0 {
		dat := orderedmap.NewOrderedMap[string, any]()
		dat.Set("current_tick", i.Tick)
		dat.Set("previous_tick", c.LastFrame)
		d.Fail(p, dat)
		return false
	}
	return true
}
