package world

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	dfworld "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type liquidProvider interface {
	Liquid(pos cube.Pos) (dfworld.Liquid, bool)
}

type additionalBlockUpdater interface {
	SetAdditionalBlock(pos cube.Pos, b dfworld.Block)
}

func TestWorldExposesSecondLayerLiquid(t *testing.T) {
	FinalizeBlockRegistry()
	pos := cube.Pos{3, 12, 5}
	c := chunk.New(BlockRegistry, cube.Range(dfworld.Overworld.Range()))
	c.SetBlock(uint8(pos.X()), int16(pos.Y()), uint8(pos.Z()), 1, BlockRegistry.BlockRuntimeID(block.Water{Depth: 8}))

	w := New(nil)
	w.AddChunk(protocol.ChunkPos{}, ChunkInfo{Chunk: c})
	liquids, ok := any(w).(liquidProvider)
	if !ok {
		t.Fatal("World does not implement the liquid provider")
	}
	liquid, ok := liquids.Liquid(pos)
	if !ok || liquid.LiquidType() != "water" {
		t.Fatalf("Liquid(%v) = %v, %t; want water", pos, liquid, ok)
	}
}

func TestWorldAdditionalBlockUpdateOverridesChunk(t *testing.T) {
	FinalizeBlockRegistry()
	pos := cube.Pos{3, 12, 5}
	c := chunk.New(BlockRegistry, cube.Range(dfworld.Overworld.Range()))
	c.SetBlock(uint8(pos.X()), int16(pos.Y()), uint8(pos.Z()), 1, BlockRegistry.BlockRuntimeID(block.Water{Depth: 8}))

	w := New(nil)
	w.AddChunk(protocol.ChunkPos{}, ChunkInfo{Chunk: c})
	updater, ok := any(w).(additionalBlockUpdater)
	if !ok {
		t.Fatal("World does not support additional block updates")
	}
	updater.SetAdditionalBlock(pos, block.Air{})
	if _, ok := w.Liquid(pos); ok {
		t.Fatal("Liquid reported after the additional layer was cleared")
	}
	updater.SetAdditionalBlock(pos, block.Water{Depth: 8})
	if liquid, ok := w.Liquid(pos); !ok || liquid.LiquidType() != "water" {
		t.Fatalf("Liquid(%v) = %v, %t; want updated water", pos, liquid, ok)
	}
}
