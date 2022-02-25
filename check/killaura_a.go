package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// KillauraA checks if a player is attacking an entity without swinging their arm.
type KillauraA struct {
	basic
	lastSwingTick uint64
}

// NewKillAuraA creates a new KillauraA check.
func NewKillAuraA() *KillauraA {
	return &KillauraA{}
}

// Name ...
func (*KillauraA) Name() (string, string) {
	return "KillAura", "A"
}

// Description ...
func (*KillauraA) Description() string {
	return "This checks if a player is attacking without swinging their arm."
}

// MaxViolations ...
func (*KillauraA) MaxViolations() float64 {
	return 15
}

// Process ...
func (k *KillauraA) Process(processor Processor, pk packet.Packet) {
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
