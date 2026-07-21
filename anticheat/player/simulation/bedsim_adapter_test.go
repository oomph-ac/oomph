package simulation

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	oomphworld "github.com/oomph-ac/oomph/anticheat/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type adapterLiquidProvider interface {
	Liquid(pos cube.Pos) (world.Liquid, bool)
}

func TestBedsimWorldProviderExposesAdditionalLiquidLayer(t *testing.T) {
	oomphworld.FinalizeBlockRegistry()
	pos := cube.Pos{3, 12, 5}
	c := chunk.New(oomphworld.BlockRegistry, cube.Range(world.Overworld.Range()))
	c.SetBlock(uint8(pos.X()), int16(pos.Y()), uint8(pos.Z()), 1, oomphworld.BlockRegistry.BlockRuntimeID(block.Water{Depth: 8}))
	w := oomphworld.New(nil)
	w.AddChunk(protocol.ChunkPos{}, oomphworld.ChunkInfo{Chunk: c})

	liquids, ok := any(bedsimWorldProvider{w: w}).(adapterLiquidProvider)
	if !ok {
		t.Fatal("bedsim world provider does not expose liquids")
	}
	if liquid, ok := liquids.Liquid(pos); !ok || liquid.LiquidType() != "water" {
		t.Fatalf("Liquid(%v) = %v, %t; want water", pos, liquid, ok)
	}
}
