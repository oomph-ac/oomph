package player

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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

	CreateWindow(windowId byte, containerType byte)
	RemoveWindow(windowId byte)

	HandleInventorySlot(pk *packet.InventorySlot)
	HandleInventoryContent(pk *packet.InventoryContent)
	HandleSingleRequest(request protocol.ItemStackRequest)
	HandleItemStackRequest(pk *packet.ItemStackRequest)
	HandleItemStackResponse(pk *packet.ItemStackResponse)

	ForceSync()
	Sync(windowID int32) bool
	SyncSlot(windowID int32, slot int) bool
}

type Inventory struct {
	items        []item.Stack
	unknownItems map[int]struct{}
	specialSlots map[int]int

	size uint32
}

func NewInventory(size uint32) *Inventory {
	return &Inventory{
		items:        make([]item.Stack, size),
		unknownItems: make(map[int]struct{}),
		specialSlots: make(map[int]int, 1),

		size: size,
	}
}

func (i *Inventory) SetSpecialSlot(slot int, specialSlot int) {
	i.specialSlots[slot] = specialSlot
}

func (i *Inventory) Slot(slot int) item.Stack {
	if specialSlot, ok := i.specialSlots[slot]; ok {
		slot = specialSlot
	}
	if slot < 0 || slot >= int(i.size) {
		panic(oerror.New("slot %d is invalid for inventory (expecting 0-%d)", slot, i.size-1))
	}
	return i.items[slot]
}

func (i *Inventory) SetSlot(slot int, it item.Stack) {
	if specialSlot, ok := i.specialSlots[slot]; ok {
		slot = specialSlot
	}
	if slot < 0 || slot >= int(i.size) {
		panic(oerror.New("slot %d is invalid for inventory (expecting 0-%d)", slot, i.size-1))
	}
	i.items[slot] = it
}

func (i *Inventory) Size() uint32 {
	return i.size
}

func (i *Inventory) Slots() []item.Stack {
	return i.items
}

func (p *Player) SetInventory(invComponent InventoryComponent) {
	p.inventory = invComponent
}

func (p *Player) Inventory() InventoryComponent {
	return p.inventory
}
