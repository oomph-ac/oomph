package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDBadPacketD = "oomph:bad_packet_d"

type BadPacketD struct {
	BaseDetection
}

func NewBadPacketD() *BadPacketD {
	d := &BadPacketD{}
	d.Type = "BadPacket"
	d.SubType = "D"

	d.Description = "Checks if a player is sending legacy block break packet (horion nuker uses this)."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *BadPacketD) ID() string {
	return DetectionIDBadPacketD
}

func (d *BadPacketD) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return true
	}

	i, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return true
	}

	dat, ok := i.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}

	if dat.ActionType == protocol.UseItemActionBreakBlock {
		d.Fail(p, nil)
		return false
	}

	return true
}
