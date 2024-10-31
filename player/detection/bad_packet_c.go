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
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 1,
		},
	}
}

func (*BadPacketC) Type() string {
	return TYPE_BAD_PACKET
}

func (*BadPacketC) SubType() string {
	return "C"
}

func (*BadPacketC) Description() string {
	return "Checks if a player is if the user is hitting themselves."
}

func (*BadPacketC) Punishable() bool {
	return true
}

func (d *BadPacketC) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketC) Detect(pk packet.Packet) {
	if t, ok := pk.(*packet.InventoryTransaction); ok {
		if dat, ok := t.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && dat.ActionType == protocol.UseItemOnEntityActionAttack && d.mPlayer.RuntimeId == dat.TargetEntityRuntimeID {
			d.mPlayer.FailDetection(d, nil)
		}
	}
}
