package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/entity"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// KillAuraB checks if a player is attacking too many entities at once.
type KillAuraB struct {
	check
	entities map[uint64]entity.Entity
}

// Name ...
func (*KillAuraB) Name() (string, string) {
	return "KillAura", "B"
}

// Description ...
func (*KillAuraB) Description() string {
	return "This checks if a player is attacking more than one entity at once."
}

// MaxViolations ...
func (*KillAuraB) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*KillAuraB) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (k *KillAuraB) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			if e, ok := processor.Entity(data.TargetEntityRuntimeID); ok {
				k.entities[data.TargetEntityRuntimeID] = e
			}
		}
	case *packet.PlayerAuthInput:
		if len(k.entities) > 1 {
			var minDist float64 = 69420
			for id, data := range k.entities {
				for subId, subData := range k.entities {
					if subId != id {
						minDist = math.Min(minDist, omath.AABBVectorDistance(data.AABB, subData.LastPosition))
					}
				}
			}
			if minDist != 69420 && minDist > 1.5 {
				processor.Flag(k, map[string]interface{}{"mD": omath.Round(minDist, 2), "entities": len(k.entities)})
			}
		}
		k.entities = map[uint64]entity.Entity{}
	}
}
