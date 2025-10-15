package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketC struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketC(p *player.Player) *BadPacketC {
	return &BadPacketC{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},
	}
}

func (*BadPacketC) Type() string {
	return TypeBadPacket
}

func (*BadPacketC) SubType() string {
	return "C"
}

func (*BadPacketC) Description() string {
	return "Checks if a player is breaking blocks in an invalid way."
}

func (*BadPacketC) Punishable() bool {
	return true
}

func (d *BadPacketC) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketC) Detect(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok && dat.ActionType == protocol.UseItemActionBreakBlock && d.mPlayer.GameMode != packet.GameTypeCreative {
			d.mPlayer.FailDetection(d)
		}
	case *packet.PlayerAction:
		switch pk.ActionType {
		case protocol.PlayerActionPredictDestroyBlock, protocol.PlayerActionStartBreak, protocol.PlayerActionCrackBreak,
			protocol.PlayerActionContinueDestroyBlock, protocol.PlayerActionAbortBreak, protocol.PlayerActionStopBreak:
			d.mPlayer.FailDetection(d)
		case protocol.PlayerActionCreativePlayerDestroyBlock:
			if d.mPlayer.GameMode != packet.GameTypeCreative {
				d.mPlayer.FailDetection(d)
			}
		}
	case *packet.PlayerAuthInput:
		if pk.InputData.Load(packet.InputFlagPerformItemInteraction) && d.mPlayer.GameMode != packet.GameTypeCreative {
			d.mPlayer.FailDetection(d)
		}
		for _, action := range pk.BlockActions {
			if action.Action == protocol.PlayerActionCreativePlayerDestroyBlock {
				d.mPlayer.FailDetection(d)
				break
			}
		}
	}
}
