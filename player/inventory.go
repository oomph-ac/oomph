package player

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// InventoryComponent is a component that handles actions related to the player's inventory.
type InventoryComponent interface {
	Helmet() world.Item
	Chestplate() world.Item
	Leggings() world.Item
	Boots() world.Item

	HandleInventorySlot(pk *packet.InventorySlot)
	HandleInventoryContent(pk *packet.InventoryContent)
}

func (p *Player) SetInventory(invComponent InventoryComponent) {
	p.inventory = invComponent
}

func (p *Player) Inventory() InventoryComponent {
	return p.inventory
}
