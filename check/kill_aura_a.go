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

func (*KillAuraA) Name() (string, string) {
	return "KillAura", "A"
}

func (*KillAuraA) Description() string {
	return "This checks if a player is attacking without swinging their arm."
}

// MaxViolations ...
func (*KillAuraA) MaxViolations() float64 {
	return 15
}

// Process ...
func (k *KillAuraA) Process(p Processor, pk packet.Packet) bool {
	switch pk := pk.(type) {
	case *packet.Animate:
		if pk.ActionType == packet.AnimateActionSwingArm {
			k.lastSwingTick = p.ClientFrame()
		}
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			currentTick := p.ClientFrame()
			tickDiff := currentTick - k.lastSwingTick
			if tickDiff > 4 {
				p.Flag(k, k.violationAfterTicks(currentTick, 600), map[string]any{
					"Tick Difference": tickDiff,
					"Current Tick":    currentTick,
					"Last Tick":       k.lastSwingTick,
				})
				return true
			}
		}
	}

	return false
}
