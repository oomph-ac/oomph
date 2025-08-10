package acknowledgement

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type SetInventoryContents struct {
	mPlayer *player.Player

	contents []protocol.ItemInstance
	windowID int32
}

func NewSetInventoryContentsACK(p *player.Player, windowID uint32, contents []protocol.ItemInstance) *SetInventoryContents {
	return &SetInventoryContents{mPlayer: p, windowID: int32(windowID), contents: contents}
}

func (ack *SetInventoryContents) Run() {
	invComponent := ack.mPlayer.Inventory()
	inv, found := invComponent.WindowFromWindowID(ack.windowID)
	if !found {
		ack.mPlayer.Log().Debug("no inventory with id found", "windowID", ack.windowID)
		return
	}

	for index, itemInstance := range ack.contents {
		if itemInstance.Stack.NetworkID == 0 {
			inv.SetSlot(index, item.NewStack(&block.Air{}, 0))
		} else {
			iStack := ack.mPlayer.ConvertToStack(itemInstance.Stack)
			inv.SetSlot(index, utils.ReadItem(itemInstance.Stack.NBTData, &iStack))
		}
	}
}

type SetInventorySlot struct {
	mPlayer *player.Player

	windowID int32
	slot     int32
	item     protocol.ItemInstance
}

func NewSetInventorySlotACK(p *player.Player, windowID uint32, slot uint32, item protocol.ItemInstance) *SetInventorySlot {
	return &SetInventorySlot{mPlayer: p, windowID: int32(windowID), slot: int32(slot), item: item}
}

func (ack *SetInventorySlot) Run() {
	invComponent := ack.mPlayer.Inventory()
	inv, found := invComponent.WindowFromWindowID(ack.windowID)
	if !found {
		ack.mPlayer.Log().Debug("no inventory with id found", "windowID", ack.windowID)
		return
	}

	if ack.item.Stack.NetworkID == 0 {
		inv.SetSlot(int(ack.slot), item.NewStack(&block.Air{}, 0))
	} else {
		iStack := ack.mPlayer.ConvertToStack(ack.item.Stack)
		inv.SetSlot(int(ack.slot), utils.ReadItem(ack.item.Stack.NBTData, &iStack))
	}
}
