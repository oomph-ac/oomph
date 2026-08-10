package player

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/anticheat/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
		if b, found := p.World().BlockRegistry().BlockByRuntimeID(p.DecodeBlockRuntimeID(uint32(it.BlockRuntimeID))); found {
			if t, ok = b.(world.Item); !ok {
				t = block.Air{}
			}
		}
	}
	if nbter, ok := t.(world.NBTer); ok && len(it.NBTData) != 0 {
		t = nbter.DecodeNBT(it.NBTData).(world.Item)
	}
	s := item.NewStack(t, int(it.Count))
	return item.ReadNBT(it.NBTData, &s).AsUnbreakable()
}

func (p *Player) InstanceFromItem(it item.Stack) protocol.ItemInstance {
	instance := utils.InstanceFromItem(p.World().BlockRegistry(), it)
	if instance.Stack.BlockRuntimeID != 0 {
		instance.Stack.BlockRuntimeID = int32(p.EncodeBlockRuntimeID(uint32(instance.Stack.BlockRuntimeID)))
	}
	return instance
}

func (p *Player) StackToItem(it protocol.ItemStack) item.Stack {
	if it.BlockRuntimeID != 0 {
		it.BlockRuntimeID = int32(p.DecodeBlockRuntimeID(uint32(it.BlockRuntimeID)))
	}
	return utils.StackToItem(p.World().BlockRegistry(), it)
}
