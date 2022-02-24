package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// KillAuraA checks if a player is attacking an entity without swinging their arm.
type KillAuraA struct {
	check
	lastSwingTick uint64
}

// NewKillAuraA creates a new KillAuraA check.
func NewKillAuraA() *KillAuraA {
	return &KillAuraA{}
}

// Name ...
func (*KillAuraA) Name() (string, string) {
	return "KillAura", "A"
}

// Description ...
func (*KillAuraA) Description() string {
	return "This checks if a player is attacking without swinging their arm."
}

// Process ...
func (k *KillAuraA) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.Animate:
		if pk.ActionType == packet.AnimateActionSwingArm {
			k.lastSwingTick = processor.ClientTick()
		}
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			tickDiff := processor.ClientTick() - k.lastSwingTick
			if tickDiff > 4 {
				processor.Flag(k, k.updateAndGetViolationAfterTicks(processor.ClientTick(), 600), map[string]interface{}{
					"Tick Difference": tickDiff,
				})
			}
		}
	}
}
