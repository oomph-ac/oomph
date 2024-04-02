package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDBadPacketC = "oomph:bad_packet_c"

type BadPacketC struct {
	BaseDetection
}

func NewBadPacketC() *BadPacketC {
	d := &BadPacketC{}
	d.Type = "BadPacket"
	d.SubType = "C"

	d.Description = "Checks if a player is if the user is hitting themselves."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *BadPacketC) ID() string {
	return DetectionIDBadPacketC
}

func (d *BadPacketC) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	// Should probably use combat handler...
	i, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return true
	}

	dat, ok := i.TransactionData.(*protocol.UseItemOnEntityTransactionData)
	if !ok {
		return true
	}

	if dat.ActionType != protocol.UseItemOnEntityActionAttack {
		return true
	}

	if p.RuntimeId == dat.TargetEntityRuntimeID {
		d.Fail(p, nil)
		return false
	}

	return true
}
