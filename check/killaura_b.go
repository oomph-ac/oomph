package check

import (
	"math"

	"github.com/justtaldevelops/oomph/entity"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/settings"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// KillAuraB checks if a player is attacking too many entities at once.
type KillAuraB struct {
	check
	Entities map[uint64]entity.Entity
}

// Name ...
func (*KillAuraB) Name() (string, string) {
	return "KillAura", "B"
}

// Description ...
func (*KillAuraB) Description() string {
	return "This checks if a player is attacking more than one entity at once."
}

// BaseSettings ...
func (*KillAuraB) BaseSettings() settings.BaseSettings {
	return settings.Settings.KillAura.B
}

// Process ...
func (k *KillAuraB) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			if e, ok := processor.Entity(data.TargetEntityRuntimeID); ok {
				k.Entities[data.TargetEntityRuntimeID] = e
			}
		}
	case *packet.PlayerAuthInput:
		if len(k.Entities) > 1 {
			var minDist float64 = 69420
			for id, data := range k.Entities {
				for subId, subData := range k.Entities {
					if subId != id {
						minDist = math.Min(minDist, omath.AABBVectorDistance(data.AABB.Translate(data.LastPosition), subData.LastPosition))
					}
				}
			}
			if minDist != 69420 && minDist > 1.5 {
				processor.Flag(k, k.updateAndGetViolationAfterTicks(processor.ClientTick(), 40), map[string]interface{}{"mD": omath.Round(minDist, 2), "entities": len(k.Entities)})
			}
		}
		k.Entities = map[uint64]entity.Entity{}
	}
}
