package component

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	inventorySizeArmour  uint32 = 4
	inventorySizePlayer  uint32 = 36
	inventorySizeHotbar  uint32 = 10
	inventorySizeOffhand uint32 = 1
	inventorySizeUI      uint32 = 54
)

type InventoryComponent struct {
	mPlayer *player.Player

	pArmour      *player.Inventory
	pInventory   *player.Inventory
	pOffhand     *player.Inventory
	pUiInventory *player.Inventory

	firstRequest   *invReq
	currentRequest *invReq

	heldSlot int32
}

func NewInventoryComponent(p *player.Player) *InventoryComponent {
	return &InventoryComponent{
		mPlayer: p,

		pInventory:   player.NewInventory(inventorySizePlayer),
		pArmour:      player.NewInventory(inventorySizeArmour),
		pOffhand:     player.NewInventory(inventorySizeOffhand),
		pUiInventory: player.NewInventory(inventorySizeUI),
	}
}

func (c *InventoryComponent) Helmet() item.Stack {
	return c.pArmour.Slot(0)
}

func (c *InventoryComponent) Chestplate() item.Stack {
	return c.pArmour.Slot(1)
}

func (c *InventoryComponent) Leggings() item.Stack {
	return c.pArmour.Slot(2)
}

func (c *InventoryComponent) Boots() item.Stack {
	return c.pArmour.Slot(3)
}

func (c *InventoryComponent) WindowFromContainerID(id int32) (*player.Inventory, bool) {
	switch id {
	case protocol.ContainerCraftingInput, protocol.ContainerCreatedOutput, protocol.ContainerCursor:
		// UI inventory.
		return c.pUiInventory, true
	case protocol.ContainerHotBar, protocol.ContainerInventory, protocol.ContainerCombinedHotBarAndInventory:
		// Hotbar 'inventory', rest of inventory, inventory when container is opened.
		return c.pInventory, true
	case protocol.ContainerOffhand:
		return c.pOffhand, true
	case protocol.ContainerArmor:
		// Armour inventory.
		return c.pArmour, true
	default:
		return nil, false
	}
}

func (c *InventoryComponent) WindowFromWindowID(id int32) (*player.Inventory, bool) {
	switch id {
	case protocol.WindowIDArmour:
		return c.pArmour, true
	case protocol.WindowIDInventory:
		return c.pInventory, true
	case protocol.WindowIDOffHand:
		return c.pOffhand, true
	case protocol.WindowIDUI:
		return c.pUiInventory, true
	default:
		return nil, false
	}
}

func (c *InventoryComponent) Sync(windowID int32) bool {
	inv, found := c.WindowFromWindowID(windowID)
	if !found {
		return false
	}

	contents := make([]protocol.ItemInstance, inv.Size())
	for i := range contents {
		contents[i] = utils.InstanceFromItem(inv.Slot(i))
	}
	_ = c.mPlayer.WritePacket(&packet.InventoryContent{
		WindowID: uint32(windowID),
		Content:  contents,
	})

	return true
}

func (c *InventoryComponent) SyncSlot(windowID int32, slot int) bool {
	inv, found := c.WindowFromWindowID(windowID)
	if !found {
		return false
	}

	_ = c.mPlayer.WritePacket(&packet.InventorySlot{
		WindowID: uint32(windowID),
		Slot:     uint32(slot),
		NewItem:  utils.InstanceFromItem(inv.Slot(slot)),
	})
	return true
}

func (c *InventoryComponent) ForceSync() {
	// Sending a mismatch transaction to the server forces the server to re-send all inventories.
	if conn := c.mPlayer.ServerConn(); conn != nil {
		_ = conn.WritePacket(&packet.InventoryTransaction{
			TransactionData: &protocol.MismatchTransactionData{},
		})
	}
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
	return c.pInventory.Slot(int(c.heldSlot))
}

func (c *InventoryComponent) HandleInventorySlot(pk *packet.InventorySlot) {
	inv, found := c.WindowFromWindowID(int32(pk.WindowID))
	if !found {
		c.mPlayer.Log().Debugf("no inventory with window id %d found", pk.WindowID)
		return
	}

	if pk.NewItem.Stack.NetworkID == 0 {
		inv.SetSlot(int(pk.Slot), item.NewStack(block.Air{}, 0))
	} else {
		iStack := utils.StackToItem(pk.NewItem.Stack)
		inv.SetSlot(int(pk.Slot), utils.ReadItem(pk.NewItem.Stack.NBTData, &iStack))
	}
}

func (c *InventoryComponent) HandleInventoryContent(pk *packet.InventoryContent) {
	inv, found := c.WindowFromWindowID(int32(pk.WindowID))
	if !found {
		c.mPlayer.Log().Debugf("no inventory with id %d found", pk.WindowID)
		return
	}

	for index, itemInstance := range pk.Content {
		if itemInstance.Stack.NetworkID == 0 {
			inv.SetSlot(index, item.NewStack(&block.Air{}, 0))
		} else {
			iStack := utils.StackToItem(itemInstance.Stack)
			inv.SetSlot(index, utils.ReadItem(itemInstance.Stack.NBTData, &iStack))
		}
	}
}

func (c *InventoryComponent) HandleSingleRequest(request protocol.ItemStackRequest) {
	c.mPlayer.Log().Debugf("received item stack request %d", request.RequestID)
	tx := newInvRequest(request.RequestID)
	for _, action := range request.Actions {
		switch action := action.(type) {
		case *protocol.TakeStackRequestAction:
			c.handleTransferRequest(tx, action.Source, action.Destination, int(action.Count))
		case *protocol.PlaceStackRequestAction:
			c.handleTransferRequest(tx, action.Source, action.Destination, int(action.Count))
		case *protocol.SwapStackRequestAction:
			c.handleSwapRequest(tx, action)
		case *protocol.DestroyStackRequestAction:
			c.handleDestroyRequest(tx, action.Source, int(action.Count), false)
		case *protocol.DropStackRequestAction:
			c.handleDestroyRequest(tx, action.Source, int(action.Count), true)
		case *protocol.MineBlockStackRequestAction:
			tx.append(newUnknownAction(c.mPlayer, fmt.Sprintf("%T", action)))
		default:
			c.mPlayer.Log().Debugf("unhandled item stack request action %T", action)
			tx.append(newUnknownAction(c.mPlayer, fmt.Sprintf("%T", action)))
		}
	}
	if len(tx.actions) > 0 {
		tx.execute()
		if c.firstRequest == nil {
			c.firstRequest = tx
			c.currentRequest = tx
		} else {
			tx.prev = c.currentRequest
			c.currentRequest.next = tx
			c.currentRequest = tx
		}
	}
}

func (c *InventoryComponent) HandleItemStackRequest(pk *packet.ItemStackRequest) {
	for _, request := range pk.Requests {
		c.HandleSingleRequest(request)
	}
}

func (c *InventoryComponent) HandleItemStackResponse(pk *packet.ItemStackResponse) {
	for _, response := range pk.Responses {
		// This should never happen, but it did :/
		if c.firstRequest == nil {
			// Here, we are going to make the server re-send what it thinks should be in the inventory to prevent any type of desync.
			c.mPlayer.Log().Debugf("cannot process response (%d) when InventoryComponent.firstRequest is nil - force syncing inventory", response.RequestID)
			//c.ForceSync()
			continue
		}

		if response.RequestID != c.firstRequest.id {
			c.mPlayer.Log().Debugf("received response for unknown request id %d", response.RequestID)
			return
		}

		if response.Status == protocol.ItemStackResponseStatusOK {
			c.mPlayer.Log().Debugf("request %d succeeded", response.RequestID)
			c.firstRequest.accept()
		} else {
			c.mPlayer.Log().Debugf("request %d failed with status %d", response.RequestID, response.Status)
			c.firstRequest.reject()
		}

		if c.firstRequest == c.currentRequest {
			c.firstRequest = nil
			c.currentRequest = nil
		} else {
			nextReq := c.firstRequest.next
			nextReq.prev = nil
			c.firstRequest.next = nil
			c.firstRequest = nextReq
		}
	}
}

func (c *InventoryComponent) handleTransferRequest(tx *invReq, src, dst protocol.StackRequestSlotInfo, count int) {
	srcInv, ok := c.WindowFromContainerID(int32(src.Container.ContainerID))
	if !ok {
		c.mPlayer.Log().Debugf("no inventory with container id %d found", src.Container.ContainerID)
		return
	}

	dstInv, ok := c.WindowFromContainerID(int32(dst.Container.ContainerID))
	if !ok {
		c.mPlayer.Log().Debugf("no inventory with container id %d found", dst.Container.ContainerID)
		return
	}

	tx.append(newInvTransferAction(
		count,
		int32(src.Container.ContainerID),
		int(src.Slot),
		srcInv.Slot(int(src.Slot)),
		int32(dst.Container.ContainerID),
		int(dst.Slot),
		dstInv.Slot(int(dst.Slot)),
		c.mPlayer,
	))
}

func (c *InventoryComponent) handleSwapRequest(tx *invReq, action *protocol.SwapStackRequestAction) {
	srcInv, ok := c.WindowFromContainerID(int32(action.Source.Container.ContainerID))
	if !ok {
		c.mPlayer.Log().Debugf("no inventory with container id %d found", action.Source.Container.ContainerID)
		return
	}

	dstInv, ok := c.WindowFromContainerID(int32(action.Destination.Container.ContainerID))
	if !ok {
		c.mPlayer.Log().Debugf("no inventory with container id %d found", action.Destination.Container.ContainerID)
		return
	}

	tx.append(newInvSwapAction(
		int32(action.Source.Container.ContainerID),
		srcInv.Slot(int(action.Source.Slot)),
		int(action.Source.Slot),
		int32(action.Destination.Container.ContainerID),
		dstInv.Slot(int(action.Destination.Slot)),
		int(action.Destination.Slot),
		c.mPlayer,
	))
}

func (c *InventoryComponent) handleDestroyRequest(tx *invReq, src protocol.StackRequestSlotInfo, count int, isDrop bool) {
	inv, foundInv := c.WindowFromContainerID(int32(src.Container.ContainerID))
	if !foundInv {
		c.mPlayer.Log().Debugf("no inventory with container id %d found", src.Container.ContainerID)
		return
	}
	tx.append(newDestroyAction(
		inv.Slot(int(src.Slot)),
		count,
		int(src.Slot),
		int32(src.Container.ContainerID),
		isDrop,
		c.mPlayer,
	))
}
