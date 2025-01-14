package component

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type InventoryComponent struct {
	mPlayer *player.Player
	armour  [4]world.Item
}

func NewInventoryComponent(p *player.Player) *InventoryComponent {
	return &InventoryComponent{mPlayer: p}
}

func (i *InventoryComponent) Helmet() world.Item {
	return i.armour[0]
}

func (i *InventoryComponent) Chestplate() world.Item {
	return i.armour[1]
}

func (i *InventoryComponent) Leggings() world.Item {
	return i.armour[2]
}

func (i *InventoryComponent) Boots() world.Item {
	return i.armour[3]
}

func (i *InventoryComponent) HandleInventorySlot(pk *packet.InventorySlot) {
	if pk.WindowID == protocol.WindowIDArmour && pk.Slot <= 3 {
		if pk.NewItem.Stack.NetworkID == 0 {
			i.armour[pk.Slot] = nil
			return
		}

		item, _ := world.ItemByRuntimeID(pk.NewItem.Stack.NetworkID, int16(pk.NewItem.Stack.MetadataValue))
		i.armour[pk.Slot] = item
	}
}

func (i *InventoryComponent) HandleInventoryContent(pk *packet.InventoryContent) {
	if pk.WindowID == protocol.WindowIDArmour {
		for index, itemInstance := range pk.Content {
			if itemInstance.Stack.NetworkID == 0 {
				i.armour[index] = nil
				continue
			}

			item, _ := world.ItemByRuntimeID(itemInstance.Stack.NetworkID, int16(itemInstance.Stack.MetadataValue))
			i.armour[index] = item
		}
	}
}
