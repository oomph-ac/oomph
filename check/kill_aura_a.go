package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// KillAuraA checks if a player is attacking an entity without swinging their arm.
type KillAuraA struct {
	lastSwingTick uint64
	basic
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

// MaxViolations ...
func (*KillAuraA) MaxViolations() float64 {
	return 15
}

// Process ...
func (k *KillAuraA) Process(p Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.Animate:
		if pk.ActionType == packet.AnimateActionSwingArm {
			k.lastSwingTick = p.ClientTick()
			p.Debug(k, map[string]interface{}{
				"Last Swing Client Tick": k.lastSwingTick,
			})
		}
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			currentTick := p.ClientTick()
			tickDiff := currentTick - k.lastSwingTick
			if tickDiff > 4 {
				p.Flag(k, k.updateAndGetViolationAfterTicks(currentTick, 600), map[string]interface{}{
					"Tick Difference": tickDiff,
					"Current Tick":    currentTick,
					"Last Tick":       k.lastSwingTick,
				})
			}
		}
	}
}
