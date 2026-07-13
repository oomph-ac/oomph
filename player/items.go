package player

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/utils"
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
	if it.BlockRuntimeID != 0 {
		b, _ := p.World().BlockRegistry().BlockByRuntimeID(p.BlockRuntimeIDFromNetwork(uint32(it.BlockRuntimeID)))
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
	instance := utils.InstanceFromItem(p.World().BlockRegistry(), it)
	if instance.Stack.BlockRuntimeID != 0 {
		instance.Stack.BlockRuntimeID = int32(p.BlockRuntimeIDToNetwork(uint32(instance.Stack.BlockRuntimeID)))
	}
	return instance
}

func (p *Player) StackToItem(it protocol.ItemStack) item.Stack {
	if it.BlockRuntimeID != 0 {
		it.BlockRuntimeID = int32(p.BlockRuntimeIDFromNetwork(uint32(it.BlockRuntimeID)))
	}
	return utils.StackToItem(p.World().BlockRegistry(), it)
}

// noinspection ALL
//
//go:linkname nbtconv_Item github.com/df-mc/dragonfly/server/internal/nbtconv.Item
func nbtconv_Item(data map[string]any, s *item.Stack) item.Stack
