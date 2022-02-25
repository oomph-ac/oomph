package check

import (
	"github.com/justtaldevelops/oomph/entity"
	"github.com/justtaldevelops/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
)

// KillAuraB checks if a player is attacking too many entities at once.
type KillAuraB struct {
	basic
	entities map[uint64]*entity.Entity
}

// NewKillAuraB creates a new KillAuraB check.
func NewKillAuraB() *KillAuraB {
	return &KillAuraB{entities: make(map[uint64]*entity.Entity)}
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
func (*KillAuraB) MaxViolations() float64 {
	return 15
}

// Process ...
func (k *KillAuraB) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			if e, ok := processor.SearchEntity(data.TargetEntityRuntimeID); ok {
				k.entities[data.TargetEntityRuntimeID] = e
			}
		}
	case *packet.PlayerAuthInput:
		if len(k.entities) > 1 {
			minDist := math.MaxFloat64
			for id, data := range k.entities {
				for subId, subData := range k.entities {
					if subId == id {
						continue
					}
					minDist = math.Min(minDist, game.AABBVectorDistance(data.AABB().Translate(data.LastPosition()), subData.LastPosition()))
				}
			}
			if minDist > 0.0 && minDist > 1.5 {
				processor.Flag(k, k.updateAndGetViolationAfterTicks(processor.ClientTick(), 40), map[string]interface{}{
					"Minimum Distance": game.Round(minDist, 2),
					"Entities":         len(k.entities),
				})
			}
		}
		k.entities = make(map[uint64]*entity.Entity)
	}
}
