package player

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/utils"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"

	_ "unsafe"
)

func (p *Player) ConvertToStack(it protocol.ItemStack) item.Stack {
	t, ok := p.items[int16(it.ItemType.NetworkID)]
	if !ok {
		t, ok = world.ItemByRuntimeID(it.NetworkID, int16(it.MetadataValue))
		if !ok {
			t = block.Air{}
		}
	}
	if it.BlockRuntimeID > 0 {
		b, _ := p.World().BlockRegistry().BlockByRuntimeID(oworld.NetworkBlockIDToRuntimeID(uint32(it.BlockRuntimeID), p.BlockNetworkIDsHashed()))
		if t, ok = b.(world.Item); !ok {
			t = block.Air{}
		}
	}
	if nbter, ok := t.(world.NBTer); ok && len(it.NBTData) != 0 {
		t = nbter.DecodeNBT(it.NBTData).(world.Item)
	}
	s := item.NewStack(t, int(it.Count))
	return nbtconv_Item(it.NBTData, &s).AsUnbreakable()
}

func (p *Player) InstanceFromItem(it item.Stack) protocol.ItemInstance {
	inst := utils.InstanceFromItem(p.World().BlockRegistry(), it)
	// The block runtime ID embedded in an item instance is a network block ID. When the session uses block
	// network ID hashes, convert the dragonfly runtime ID to its hash before it is sent to the client.
	if p.BlockNetworkIDsHashed() && inst.Stack.BlockRuntimeID != 0 {
		inst.Stack.BlockRuntimeID = int32(oworld.RuntimeIDToNetworkBlockID(uint32(inst.Stack.BlockRuntimeID), true))
	}
	return inst
}

func (p *Player) StackToItem(it protocol.ItemStack) item.Stack {
	// The block runtime ID of an item stack received over the network is a network block ID. Convert it to a
	// dragonfly runtime ID when the session uses block network ID hashes.
	if p.BlockNetworkIDsHashed() && it.BlockRuntimeID != 0 {
		it.BlockRuntimeID = int32(oworld.NetworkBlockIDToRuntimeID(uint32(it.BlockRuntimeID), true))
	}
	return utils.StackToItem(p.World().BlockRegistry(), it)
}

// noinspection ALL
//
//go:linkname nbtconv_Item github.com/df-mc/dragonfly/server/internal/nbtconv.Item
func nbtconv_Item(data map[string]any, s *item.Stack) item.Stack
