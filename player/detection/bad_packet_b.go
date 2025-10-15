package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketB(p *player.Player) *BadPacketB {
	return &BadPacketB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 1,
		},
	}
}

func (*BadPacketB) Type() string {
	return TypeBadPacket
}

func (*BadPacketB) SubType() string {
	return "B"
}

func (*BadPacketB) Description() string {
	return "Checks if a player is if the user is hitting themselves."
}

func (*BadPacketB) Punishable() bool {
	return true
}

func (d *BadPacketB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketB) Detect(pk packet.Packet) {
	if t, ok := pk.(*packet.InventoryTransaction); ok {
		if dat, ok := t.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && dat.ActionType == protocol.UseItemOnEntityActionAttack && d.mPlayer.RuntimeId == dat.TargetEntityRuntimeID {
			d.mPlayer.FailDetection(d)
		}
	}
}
