package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// KillAuraA checks if a player is attacking an entity without swinging their arm.
type KillAuraA struct {
	check
	lastSwingTick uint64
}

// Name ...
func (*KillAuraA) Name() (string, string) {
	return "KillAura", "A"
}

// Description ...
func (*KillAuraA) Description() string {
	return "This checks if a player is attacking without swinging their arm."
}

// MaxViolations ...
func (*KillAuraA) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*KillAuraA) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (k *KillAuraA) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.Animate:
		if pk.ActionType == packet.AnimateActionSwingArm {
			k.lastSwingTick = processor.Tick()
		}
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			tickDiff := processor.Tick() - k.lastSwingTick
			processor.Debug(k, map[string]interface{}{"tickDiff": tickDiff})
			if tickDiff > 4 {
				processor.Flag(k, map[string]interface{}{"tickDiff": tickDiff})
			}
		}
	}
}
