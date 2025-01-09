package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/world"
)

var ironTrapdoorHash = block.NextHash()

func init() {
	for _, dir := range cube.Directions() {
		world.RegisterBlock(IronTrapdoor{Facing: dir, Open: false, Top: false})
		world.RegisterBlock(IronTrapdoor{Facing: dir, Open: false, Top: true})
		world.RegisterBlock(IronTrapdoor{Facing: dir, Open: true, Top: false})
		world.RegisterBlock(IronTrapdoor{Facing: dir, Open: true, Top: true})
	}
}

type IronTrapdoor struct {
	Facing    cube.Direction
	Open, Top bool
}

func (t IronTrapdoor) EncodeBlock() (string, map[string]any) {
	return "minecraft:iron_trapdoor", map[string]any{
		"direction":       fuckDirection(t.Facing),
		"open_bit":        boolByte(t.Open),
		"upside_down_bit": boolByte(t.Top),
	}
}

func (t IronTrapdoor) Hash() (uint64, uint64) {
	return ironTrapdoorHash, uint64(t.Facing) | uint64(boolByte(t.Open))<<32 | uint64(boolByte(t.Top))<<33
}

func (t IronTrapdoor) Model() world.BlockModel {
	return model.Trapdoor{
		Facing: t.Facing,
		Open:   t.Open,
		Top:    t.Top,
	}
}
