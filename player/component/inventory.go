package component

import (
	"fmt"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/recipe"
	"github.com/df-mc/dragonfly/server/world"
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

const (
	craftingSmallUI     = 4
	craftingSmallOffset = 28
	craftingBigUI       = 9
	craftingBigOffset   = 32
)

type InventoryComponent struct {
	mPlayer *player.Player

	pArmour      *player.Inventory
	pInventory   *player.Inventory
	pOffhand     *player.Inventory
	pUiInventory *player.Inventory

	altOpenWindow   *player.Inventory
	altOpenWindowId byte

	firstRequest   *invReq
	currentRequest *invReq

	heldSlot int32
}

func NewInventoryComponent(p *player.Player) *InventoryComponent {
	c := &InventoryComponent{
		mPlayer: p,

		pInventory:   player.NewInventory(inventorySizePlayer),
		pArmour:      player.NewInventory(inventorySizeArmour),
		pOffhand:     player.NewInventory(inventorySizeOffhand),
		pUiInventory: player.NewInventory(inventorySizeUI),
	}
	c.pOffhand.SetSpecialSlot(1, 0)

	return c
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
		return c.altOpenWindow, c.altOpenWindow != nil
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
		return c.altOpenWindow, c.altOpenWindow != nil
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

func (c *InventoryComponent) CreateWindow(windowId byte, containerType byte) {
	if containerType == 255 {
		return
	}

	switch windowId {
	case protocol.WindowIDInventory, protocol.WindowIDOffHand, protocol.WindowIDArmour, protocol.WindowIDUI:
		return
	default:
		if c.altOpenWindow != nil {
			c.mPlayer.Log().Debug("creating new window, but alternative is already open", "currWindowID", c.altOpenWindowId, "newWindowID", windowId)
		}

		// TODO: Determine the actual sizes of these inventory based on the container type. CBA to do for now, but this should work.
		c.mPlayer.Log().Debug("created container inventory", "windowID", windowId, "containerType", containerType)
		c.altOpenWindow = player.NewInventory(64)
		c.altOpenWindowId = windowId
	}
}

func (c *InventoryComponent) RemoveWindow(windowId byte) {
	if windowId != c.altOpenWindowId {
		return
	}
	c.altOpenWindow = nil
	c.mPlayer.Log().Debug("removed container inventory", "windowID", windowId)
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
		c.mPlayer.Log().Debug("no inventory with window id found", "windowID", pk.WindowID)
		return
	}

	if pk.NewItem.Stack.NetworkID == 0 {
		inv.SetSlot(int(pk.Slot), item.NewStack(&block.Air{}, 0))
	} else {
		iStack := utils.StackToItem(pk.NewItem.Stack)
		inv.SetSlot(int(pk.Slot), utils.ReadItem(pk.NewItem.Stack.NBTData, &iStack))
	}
}

func (c *InventoryComponent) HandleInventoryContent(pk *packet.InventoryContent) {
	inv, found := c.WindowFromWindowID(int32(pk.WindowID))
	if !found {
		c.mPlayer.Log().Debug("no inventory with id found", "windowID", pk.WindowID)
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
	c.mPlayer.Log().Debug("received item stack request", "requestID", request.RequestID)
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
		case *protocol.CraftRecipeStackRequestAction:
			c.handleCraftStackRequest(tx, action)
		case *protocol.CraftResultsDeprecatedStackRequestAction, *protocol.ConsumeStackRequestAction:
			tx.append(newNopAction())
		default:
			c.mPlayer.Log().Debug("unhandled item stack request action", "actionType", fmt.Sprintf("%T", action))
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
			c.mPlayer.Log().Debug("cannot process response when InventoryComponent.firstRequest is nil - force syncing inventory", "requestID", response.RequestID)
			//c.ForceSync()
			continue
		}

		if response.RequestID != c.firstRequest.id {
			c.mPlayer.Log().Debug("received response for unknown request id", "requestID", response.RequestID)
			return
		}

		if response.Status == protocol.ItemStackResponseStatusOK {
			c.mPlayer.Log().Debug("request succeeded", "requestID", response.RequestID)
			c.firstRequest.accept()
		} else {
			c.mPlayer.Log().Debug("request failed", "requestID", response.RequestID, "status", response.Status)
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

func (c *InventoryComponent) handleCraftStackRequest(tx *invReq, action *protocol.CraftRecipeStackRequestAction) {
	recp, ok := c.mPlayer.Recipies[action.RecipeNetworkID]
	if !ok {
		c.mPlayer.Log().Debug("no recipe found", "recipeNetworkID", action.RecipeNetworkID)
		return
	}

	var (
		recpBlock  string
		recpInput  []recipe.Item
		recpOutput []item.Stack
	)
	switch recp := recp.(type) {
	case *protocol.ShapedRecipe:
		recpBlock = recp.Block
		recpInput = make([]recipe.Item, len(recp.Input))
		for index, desc := range recp.Input {
			switch d := desc.Descriptor.(type) {
			case *protocol.DefaultItemDescriptor:
				if i, ok := world.ItemByRuntimeID(int32(d.NetworkID), d.MetadataValue); ok {
					c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "item found %T for index %d", i, index)
					recpInput[index] = item.NewStack(i, int(desc.Count))
				} else {
					c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "no item found %d %d (index %d)", d.NetworkID, d.MetadataValue, index)
				}
			case *protocol.ItemTagItemDescriptor:
				c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "item tag found %s for index %d", d.Tag, index)
				recpInput[index] = recipe.NewItemTag(d.Tag, int(desc.Count))
			}
		}
		recpOutput = make([]item.Stack, len(recp.Output))
		for index, stack := range recp.Output {
			recpOutput[index] = utils.StackToItem(stack)
		}
	case *protocol.ShapelessRecipe:
		recpBlock = recp.Block
		recpInput = make([]recipe.Item, len(recp.Input))
		for index, desc := range recp.Input {
			switch d := desc.Descriptor.(type) {
			case *protocol.DefaultItemDescriptor:
				if i, ok := world.ItemByRuntimeID(int32(d.NetworkID), d.MetadataValue); ok {
					c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "item found %T for index %d", i, index)
					recpInput[index] = item.NewStack(i, int(desc.Count))
				} else {
					c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "no item found %d %d (index %d)", d.NetworkID, d.MetadataValue, index)
				}
			case *protocol.ItemTagItemDescriptor:
				recpInput[index] = recipe.NewItemTag(d.Tag, int(desc.Count))
				c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "item tag found %s for index %d", d.Tag, index)
			}
		}
		recpOutput = make([]item.Stack, len(recp.Output))
		for index, stack := range recp.Output {
			recpOutput[index] = utils.StackToItem(stack)
		}
	default:
		fmt.Println("not shaped or shapeless recipe")
		return
	}

	if recpBlock != "crafting_table" {
		c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "recp is %s not crafting table", recpBlock)
		return
	}
	if action.NumberOfCrafts < 1 {
		c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "invalid num crafts %d", action.NumberOfCrafts)
		return
	}

	var craftingTableSize, craftingTableOffset uint32
	if c.altOpenWindow != nil {
		craftingTableSize, craftingTableOffset = craftingBigUI, craftingBigOffset
	} else {
		craftingTableSize, craftingTableOffset = craftingSmallUI, craftingSmallOffset
	}
	consumed := make([]bool, craftingTableSize)
	craftingInv := c.pUiInventory

	craftAllowed := true
	for _, expected := range recpInput {
		if expected == nil || expected.Empty() {
			continue
		}

		var processed bool
		for slot := craftingTableOffset; slot < craftingTableOffset+craftingTableSize; slot++ {
			if consumed[slot-craftingTableOffset] {
				// We've already consumed this slot, skip it.
				continue
			}
			has := craftingInv.Slot(int(slot))
			if has.Empty() != expected.Empty() || has.Count() < expected.Count()*int(action.NumberOfCrafts) {
				// We can't process this item, as it's not a part of the recipe.
				continue
			}
			if !crafting_matchingStacks(has, expected) {
				// Not the same item.
				continue
			}
			processed, consumed[slot-craftingTableOffset] = true, true
			tx.append(newDestroyAction(
				has,
				expected.Count()*int(action.NumberOfCrafts),
				int(slot),
				protocol.ContainerCraftingInput,
				false,
				c.mPlayer,
			))
			//st := has.Grow(-expected.Count() * int(action.NumberOfCrafts))
			//craftingInv.SetSlot(int(slot), st)
			break
		}
		if !processed {
			c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "recipe %v: could not consume expected item: %v", action.RecipeNetworkID, expected)
			craftAllowed = false
			//break
		}
	}

	if craftAllowed {
		result := crafting_repeatStacks(recpOutput, int(action.NumberOfCrafts))
		for _, out := range result {
			tx.append(newCreateAction(
				50,
				protocol.ContainerCreatedOutput,
				out,
				c.mPlayer,
			))
			c.mPlayer.Dbg.Notify(player.DebugModeCrafting, true, "created %v", out)
		}
	}
}

func (c *InventoryComponent) handleTransferRequest(tx *invReq, src, dst protocol.StackRequestSlotInfo, count int) {
	srcInv, ok := c.WindowFromContainerID(int32(src.Container.ContainerID))
	if !ok {
		c.mPlayer.Log().Debug("no inventory with container id found", "containerID", src.Container.ContainerID)
		return
	}

	dstInv, ok := c.WindowFromContainerID(int32(dst.Container.ContainerID))
	if !ok {
		c.mPlayer.Log().Debug("no inventory with container id found", "containerID", dst.Container.ContainerID)
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
		c.mPlayer.Log().Debug("no inventory with container id found", "containerID", action.Source.Container.ContainerID)
		return
	}

	dstInv, ok := c.WindowFromContainerID(int32(action.Destination.Container.ContainerID))
	if !ok {
		c.mPlayer.Log().Debug("no inventory with container id found", "containerID", action.Destination.Container.ContainerID)
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
		c.mPlayer.Log().Debug("no inventory with container id found", "containerID", src.Container.ContainerID)
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

// noinspection ALL
//
//go:linkname crafting_matchingStacks github.com/df-mc/dragonfly/server/session.matchingStacks
func crafting_matchingStacks(a, b recipe.Item) bool

// noinspection ALL
//
//go:linkname crafting_grow github.com/df-mc/dragonfly/server/session.grow
func crafting_grow(recipe.Item, int) recipe.Item

// noinspection ALL
//
//go:linkname crafting_repeatStacks github.com/df-mc/dragonfly/server/session.repeatStacks
func crafting_repeatStacks([]item.Stack, int) []item.Stack
