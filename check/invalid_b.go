package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type InvalidB struct {
	basic
}

func NewInvalidB() *InvalidB {
	return &InvalidB{}
}

func (*InvalidB) Name() (string, string) {
	return "Invalid", "B"
}

func (*InvalidB) Description() string {
	return "This checks if a player is sending the wrong type of packet to break blocks."
}

func (*InvalidB) MaxViolations() float64 {
	return 1.0
}

func (b *InvalidB) Process(p Processor, pk packet.Packet) bool {
	i, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return false
	}

	td, ok := i.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return false
	}

	if td.ActionType != protocol.UseItemActionBreakBlock {
		return false
	}

	p.Flag(b, 1, map[string]any{})

	return false
}
