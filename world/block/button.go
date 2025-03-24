package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/world/blockmodel"
)

var buttonHash = block.NextHash()

func init() {
	for _, b := range allButtonBlocks() {
		world.RegisterBlock(b)
	}
}

type Button struct {
	Block world.Block

	Direction cube.Face
	Pressed   bool
}

func (b Button) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + encodeButtonType(b.Block) + "_button", map[string]any{"button_pressed_bit": boolByte(b.Pressed), "facing_direction": int32(b.Direction)}
}

func (b Button) Hash() (uint64, uint64) {
	return buttonHash, world.BlockHash(b.Block) | uint64(boolByte(b.Pressed))<<32 | uint64(b.Direction)<<33
}

func (b Button) Model() world.BlockModel {
	return blockmodel.NoCollisionNotSolid{}
}

func allButtonBlocks() (blocks []world.Block) {
	for _, dir := range cube.Faces() {
		for _, woodType := range block.WoodTypes() {
			blocks = append(blocks, Button{Block: block.Planks{Wood: woodType}, Direction: dir, Pressed: false})
			blocks = append(blocks, Button{Block: block.Planks{Wood: woodType}, Direction: dir, Pressed: true})
		}
		blocks = append(blocks, Button{Block: block.Stone{}, Direction: dir, Pressed: false})
		blocks = append(blocks, Button{Block: block.Stone{}, Direction: dir, Pressed: true})
		blocks = append(blocks, Button{Block: block.Blackstone{Type: block.PolishedBlackstone()}, Direction: dir, Pressed: false})
		blocks = append(blocks, Button{Block: block.Blackstone{Type: block.PolishedBlackstone()}, Direction: dir, Pressed: true})
	}
	return
}

func encodeButtonType(b world.Block) string {
	switch b := b.(type) {
	case block.Planks:
		if n := b.Wood.String(); n == "oak" {
			return "wooden"
		} else {
			return n
		}
	case block.Stone:
		return "stone"
	case block.Blackstone:
		if b.Type == block.PolishedBlackstone() {
			return b.Type.String()
		}
	}
	panic("invalid block for button type")
}
