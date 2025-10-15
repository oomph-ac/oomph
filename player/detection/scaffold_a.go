package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ScaffoldA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_ScaffoldA(p *player.Player) *ScaffoldA {
	return &ScaffoldA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 5,
		},
	}
}

func (*ScaffoldA) Type() string {
	return TypeScaffold
}

func (*ScaffoldA) SubType() string {
	return "A"
}

func (*ScaffoldA) Description() string {
	return "Checks if the click vector is zero during an initial right click input."
}

func (*ScaffoldA) Punishable() bool {
	return true
}

func (d *ScaffoldA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *ScaffoldA) Detect(pk packet.Packet) {
	invPk, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return
	}
	trData, ok := invPk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return
	}
	if !d.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) {
		return
	}
	if trData.ClickedPosition.LenSqr() == 0 && trData.TriggerType == protocol.TriggerTypePlayerInput {
		d.mPlayer.FailDetection(d)
	}
}
