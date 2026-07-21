package component

import (
	"log/slog"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestUpdateBlockQueuesSecondLayerSeparately(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{}, nil)
	p.Ready = true
	updater := NewWorldUpdaterComponent(p)
	pos := cube.Pos{1, 2, 3}
	networkRuntimeID := uint32(42)

	updater.HandleUpdateBlock(&packet.UpdateBlock{
		Position:          protocol.BlockPos{int32(pos.X()), int32(pos.Y()), int32(pos.Z())},
		NewBlockRuntimeID: networkRuntimeID,
		Layer:             1,
	})

	want := p.DecodeBlockRuntimeID(networkRuntimeID)
	if got := updater.batchedBlockUpdates.AdditionalBlocks()[pos]; got != want {
		t.Fatalf("additional runtime ID = %d, want %d", got, want)
	}
	if _, ok := updater.batchedBlockUpdates.Blocks()[pos]; ok {
		t.Fatal("second-layer update was also queued on the primary layer")
	}
}

func TestUpdateSubChunkExtraQueuesSecondLayerSeparately(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{}, nil)
	p.Ready = true
	updater := NewWorldUpdaterComponent(p)
	pos := cube.Pos{4, 5, 6}
	networkRuntimeID := uint32(43)

	updater.HandleUpdateSubChunkBlocks(&packet.UpdateSubChunkBlocks{
		Extra: []protocol.BlockChangeEntry{{
			BlockPos:       protocol.BlockPos{int32(pos.X()), int32(pos.Y()), int32(pos.Z())},
			BlockRuntimeID: networkRuntimeID,
		}},
	})

	want := p.DecodeBlockRuntimeID(networkRuntimeID)
	if got := updater.batchedBlockUpdates.AdditionalBlocks()[pos]; got != want {
		t.Fatalf("additional runtime ID = %d, want %d", got, want)
	}
	if _, ok := updater.batchedBlockUpdates.Blocks()[pos]; ok {
		t.Fatal("extra sub-chunk update was also queued on the primary layer")
	}
}
