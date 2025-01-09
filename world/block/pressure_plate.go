package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/world/blockmodel"
)

var pressurePlateHash = block.NextHash()

func init() {
	for _, plate := range allPressurePlates() {
		world.RegisterBlock(plate)
	}
}

type PressurePlate struct {
	RedstoneSignal int32
	Block          world.Block
}

func (pp PressurePlate) Hash() (uint64, uint64) {
	return pressurePlateHash, world.BlockHash(pp.Block) | uint64(pp.RedstoneSignal)<<32
}

func (pp PressurePlate) Model() world.BlockModel {
	return blockmodel.NoCollisionSolid{}
}

func (pp PressurePlate) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + encodePressurePlateBlock(pp.Block) + "_pressure_plate", map[string]any{"redstone_signal": pp.RedstoneSignal}
}

func PressurePlateBlocks() (blocks []world.Block) {
	for _, woodType := range block.WoodTypes() {
		blocks = append(blocks, block.Planks{Wood: woodType})
	}
	blocks = append(
		blocks,
		block.Stone{},
		block.Blackstone{Type: block.PolishedBlackstone()},
		block.Gold{},
		block.Iron{},
	)
	return
}

func allPressurePlates() (pressurePlates []world.Block) {
	for _, b := range PressurePlateBlocks() {
		for i := int32(0); i <= 15; i++ {
			pressurePlates = append(pressurePlates, PressurePlate{Block: b, RedstoneSignal: i})
		}
	}
	return
}

func encodePressurePlateBlock(b world.Block) string {
	switch b := b.(type) {
	case block.Planks:
		if b.Wood == block.OakWood() {
			return "wooden"
		}
		return b.Wood.String()
	case block.Stone:
		return "stone"
	case block.Blackstone:
		return block.PolishedBlackstone().String()
	case block.Gold:
		return "light_weighted"
	case block.Iron:
		return "heavy_weighted"
	}

	panic("invalid block used for pressure plates")
}
