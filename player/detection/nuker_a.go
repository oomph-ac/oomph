package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type NukerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_NukerA(p *player.Player) *NukerA {
	return &NukerA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
		},
	}
}

func (*NukerA) Type() string {
	return TypeNuker
}

func (*NukerA) SubType() string {
	return "A"
}

func (*NukerA) Description() string {
	return "Checks if a player sends the wrong packet for breaking blocks."
}

func (*NukerA) Punishable() bool {
	return true
}

func (d *NukerA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *NukerA) Detect(pk packet.Packet) {
	invPk, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return
	}
	trDat, ok := invPk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return
	}
	if trDat.ActionType == protocol.UseItemActionBreakBlock && (d.mPlayer.GameMode == packet.GameTypeSurvival || d.mPlayer.GameMode == packet.GameTypeAdventure) {
		d.mPlayer.FailDetection(d)
	}
}
