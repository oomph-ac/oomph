package component

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	inventorySizeArmour uint32 = 4
	inventorySizePlayer uint32 = 36
	inventorySizeHotbar uint32 = 10
)

type InventoryComponent struct {
	mPlayer    *player.Player
	armour     [4]world.Item
	pInventory [36]item.Stack
	heldSlot   int32
}

func NewInventoryComponent(p *player.Player) *InventoryComponent {
	emptyInv := [36]item.Stack{}
	for index := range emptyInv {
		emptyInv[index] = item.NewStack(&block.Air{}, 0)
	}
	return &InventoryComponent{
		mPlayer:    p,
		pInventory: emptyInv,
	}
}

func (c *InventoryComponent) Helmet() world.Item {
	return c.armour[0]
}

func (c *InventoryComponent) Chestplate() world.Item {
	return c.armour[1]
}

func (c *InventoryComponent) Leggings() world.Item {
	return c.armour[2]
}

func (c *InventoryComponent) Boots() world.Item {
	return c.armour[3]
}

func (c *InventoryComponent) Slot(slot int) item.Stack {
	validatePlayerInventorySlot(slot)
	return c.pInventory[slot]
}

func (c *InventoryComponent) SetSlot(slot int, i item.Stack) {
	validatePlayerInventorySlot(slot)
	c.pInventory[slot] = i
}

func (c *InventoryComponent) HeldSlot() int32 {
	return c.heldSlot
}

func (c *InventoryComponent) SetHeldSlot(heldSlot int32) {
	if heldSlot < 0 || heldSlot >= int32(inventorySizeHotbar) {
		c.mPlayer.Disconnect(game.ErrorInvalidInventorySlot)
		return
	}
	c.heldSlot = heldSlot
}

func (c *InventoryComponent) Holding() item.Stack {
	return c.pInventory[c.heldSlot]
}

func (c *InventoryComponent) HandleInventorySlot(pk *packet.InventorySlot) {
	if pk.WindowID == protocol.WindowIDArmour && pk.Slot < inventorySizeArmour {
		if pk.NewItem.Stack.NetworkID == 0 {
			c.armour[pk.Slot] = nil
			return
		}

		item, _ := world.ItemByRuntimeID(pk.NewItem.Stack.NetworkID, int16(pk.NewItem.Stack.MetadataValue))
		c.armour[pk.Slot] = item
	} else if pk.WindowID == protocol.WindowIDInventory && pk.Slot < inventorySizePlayer {
		if pk.NewItem.Stack.NetworkID == 0 {
			c.pInventory[pk.Slot] = item.NewStack(&block.Air{}, 0)
			return
		}

		if i, found := world.ItemByRuntimeID(pk.NewItem.Stack.NetworkID, int16(pk.NewItem.Stack.MetadataValue)); found {
			c.pInventory[pk.Slot] = item.NewStack(i, int(pk.NewItem.Stack.Count))
		} else {
			c.mPlayer.Message("could not find item with runtime id %d w/ meta %d", pk.NewItem.Stack.NetworkID, pk.NewItem.Stack.MetadataValue)
		}
	}
}

func (c *InventoryComponent) HandleInventoryContent(pk *packet.InventoryContent) {
	if pk.WindowID == protocol.WindowIDArmour {
		for index, itemInstance := range pk.Content {
			if itemInstance.Stack.NetworkID == 0 {
				c.armour[index] = nil
				continue
			}

			item, _ := world.ItemByRuntimeID(itemInstance.Stack.NetworkID, int16(itemInstance.Stack.MetadataValue))
			c.armour[index] = item
		}
	} else if pk.WindowID == protocol.WindowIDInventory {
		for index, itemInstance := range pk.Content {
			if itemInstance.Stack.NetworkID == 0 {
				c.pInventory[index] = item.NewStack(&block.Air{}, 0)
				continue
			}

			if i, found := world.ItemByRuntimeID(itemInstance.Stack.NetworkID, int16(itemInstance.Stack.MetadataValue)); found {
				c.pInventory[index] = item.NewStack(i, int(itemInstance.Stack.Count))
			} else {
				c.mPlayer.Message("could not find item with runtime id %d w/ meta %d", itemInstance.Stack.NetworkID, itemInstance.Stack.MetadataValue)
			}
		}
	}
}

func validatePlayerInventorySlot(slot int) {
	if slot < 0 || slot >= int(inventorySizePlayer) {
		panic(oerror.New("slot %d is invalid for player inventory (expecting 0-35)", slot))
	}
}
