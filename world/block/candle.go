package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/world/blockmodel"
)

var candleHash = block.NextHash()

func init() {
	for _, c := range item.Colours() {
		for i := range 4 {
			world.RegisterBlock(Candle{Colour: c, Count: int32(i + 1), Lit: false})
			world.RegisterBlock(Candle{Colour: c, Count: int32(i + 1), Lit: true})
		}
	}
}

type Candle struct {
	Colour item.Colour
	Count  int32 // index is "candles"
	Lit    bool  // index is "lit"
}

func (c Candle) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + c.Colour.String() + "_candle", map[string]any{"candles": c.Count - 1, "lit": c.Lit}
}

func (c Candle) Hash() (uint64, uint64) {
	return candleHash, uint64(c.Colour.Uint8())<<32 | uint64(c.Count-1)<<36 | uint64(boolByte(c.Lit))<<40
}

func (c Candle) Model() world.BlockModel {
	return blockmodel.Candle{Count: c.Count}
}
