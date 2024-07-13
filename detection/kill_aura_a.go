package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDKillAuraA = "oomph:kill_aura_a"

type KillAuraA struct {
	BaseDetection
}

func NewKillAuraA() *KillAuraA {
	d := &KillAuraA{}
	d.Type = "KillAura"
	d.SubType = "A"

	d.Description = "Detects if a player is attacking without swinging their arm"
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 1
	d.MaxBuffer = 1
	return d
}

func (d *KillAuraA) ID() string {
	return DetectionIDKillAuraA
}

func (d *KillAuraA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.CombatMode != player.AuthorityModeSemi {
		return true
	}
	tpk, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return true
	}

	c := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)
	if data, ok := tpk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
		currentTick := p.ClientFrame
		tickDiff := currentTick - c.LastSwingTick
		e := p.Handler(handler.HandlerIDEffects).(*handler.EffectsHandler)
		var maxTickDiff int64 = 10
		if miningFatigue, ok := e.Get(packet.EffectMiningFatigue); ok {
			maxTickDiff += int64(miningFatigue.Level())
		}
		if tickDiff > maxTickDiff {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("tick_diff", tickDiff)
			data.Set("current_tick", currentTick)
			data.Set("last_tick", c.LastSwingTick)
			d.Fail(p, data)
			return false
		}
	}
	return true
}
