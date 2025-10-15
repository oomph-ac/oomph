package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketG struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketG(p *player.Player) *BadPacketG {
	return &BadPacketG{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},
	}
}

func (*BadPacketG) Type() string {
	return TypeBadPacket
}

func (*BadPacketG) SubType() string {
	return "G"
}

func (*BadPacketG) Description() string {
	return "Checks if the player is sending an invalid block-face value."
}

func (*BadPacketG) Punishable() bool {
	return true
}

func (d *BadPacketG) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketG) Detect(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		trDat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
		if !ok {
			return
		}
		if trDat.ActionType != protocol.UseItemActionClickAir && !utils.IsBlockFaceValid(trDat.BlockFace) {
			d.mPlayer.FailDetection(d)
		}
	case *packet.PlayerAuthInput:
		if pk.InputData.Load(packet.InputFlagPerformItemInteraction) {
			if !utils.IsBlockFaceValid(pk.ItemInteractionData.BlockFace) {
				d.mPlayer.FailDetection(d)
			}
		}
		if pk.InputData.Load(packet.InputFlagPerformBlockActions) {
			for _, action := range pk.BlockActions {
				if action.Action != protocol.PlayerActionAbortBreak && !utils.IsBlockFaceValid(action.Face) {
					d.mPlayer.FailDetection(d)
				}
			}
		}
	}
}
