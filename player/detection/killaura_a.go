package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type KillauraA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_KillauraA(p *player.Player) *KillauraA {
	return &KillauraA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 1,
		},
	}
}

func (*KillauraA) Type() string {
	return TypeKillaura
}

func (*KillauraA) SubType() string {
	return "A"
}

func (*KillauraA) Description() string {
	return "Detects if a player is attacking without swinging their arm"
}

func (*KillauraA) Punishable() bool {
	return true
}

func (d *KillauraA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *KillauraA) Detect(pk packet.Packet) {
	tpk, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return
	}

	lastSwung := d.mPlayer.Combat().LastSwing()
	if data, ok := tpk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
		currentTick := d.mPlayer.SimulationFrame
		tickDiff := int64(currentTick) - lastSwung
		var maxTickDiff int64 = 10
		if miningFatigue, ok := d.mPlayer.Effects().Get(packet.EffectMiningFatigue); ok {
			maxTickDiff += int64(miningFatigue.Amplifier)
		}

		if tickDiff > maxTickDiff {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("tick_diff", tickDiff)
			data.Set("current_tick", currentTick)
			data.Set("last_tick", lastSwung)
			d.mPlayer.FailDetection(d, data)
		}
	}
}
