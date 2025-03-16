package player

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// InventoryComponent is a component that handles actions related to the player's inventory.
type InventoryComponent interface {
	Helmet() item.Stack
	Chestplate() item.Stack
	Leggings() item.Stack
	Boots() item.Stack

	WindowFromContainerID(int32) (*Inventory, bool)
	WindowFromWindowID(int32) (*Inventory, bool)

	HeldSlot() int32
	SetHeldSlot(int32)
	Holding() item.Stack

	HandleInventorySlot(pk *packet.InventorySlot)
	HandleInventoryContent(pk *packet.InventoryContent)
	HandleItemStackRequest(pk *packet.ItemStackRequest)
	HandleItemStackResponse(pk *packet.ItemStackResponse)
}

type Inventory struct {
	items []item.Stack
	size  uint32
}

func NewInventory(size uint32) *Inventory {
	return &Inventory{
		items: make([]item.Stack, size),
		size:  size,
	}
}

func (i *Inventory) Slot(slot int) item.Stack {
	if slot < 0 || slot >= int(i.size) {
		panic(oerror.New("slot %d is invalid for inventory (expecting 0-%d)", slot, i.size-1))
	}
	return i.items[slot]
}

func (i *Inventory) SetSlot(slot int, it item.Stack) {
	if slot < 0 || slot >= int(i.size) {
		panic(oerror.New("slot %d is invalid for inventory (expecting 0-%d)", slot, i.size-1))
	}
	i.items[slot] = it
}

func (p *Player) SetInventory(invComponent InventoryComponent) {
	p.inventory = invComponent
}

func (p *Player) Inventory() InventoryComponent {
	return p.inventory
}
