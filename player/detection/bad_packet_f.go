package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketF struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketF(p *player.Player) *BadPacketF {
	return &BadPacketF{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
			TrustDuration: -1,
		},
	}
}

func (*BadPacketF) Type() string {
	return TypeBadPacket
}

func (*BadPacketF) SubType() string {
	return "F"
}

func (*BadPacketF) Description() string {
	return "Checks if the player's TriggerType is valid"
}

func (*BadPacketF) Punishable() bool {
	return true
}

func (d *BadPacketF) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketF) Detect(pk packet.Packet) {
	tr, ok := pk.(*packet.InventoryTransaction)
	if !ok || !d.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) {
		return
	}

	switch dat := tr.TransactionData.(type) {
	case *protocol.ReleaseItemTransactionData:
		d.checkHotbarSlot(dat.HotBarSlot)
	case *protocol.UseItemOnEntityTransactionData:
		d.checkHotbarSlot(dat.HotBarSlot)
	case *protocol.UseItemTransactionData:
		d.checkHotbarSlot(dat.HotBarSlot)
		if dat.ActionType != protocol.UseItemActionClickBlock {
			return
		}
		if dat.TriggerType != protocol.TriggerTypePlayerInput && dat.TriggerType != protocol.TriggerTypeSimulationTick {
			d.mPlayer.FailDetection(d, nil)
		}
		if dat.ClientPrediction != protocol.ClientPredictionFailure && dat.ClientPrediction != protocol.ClientPredictionSuccess {
			d.mPlayer.FailDetection(d, nil)
		}
	}
}

func (d *BadPacketF) checkHotbarSlot(slot int32) {
	if slot < 0 || slot >= 9 {
		d.mPlayer.FailDetection(d, nil)
	}
}
