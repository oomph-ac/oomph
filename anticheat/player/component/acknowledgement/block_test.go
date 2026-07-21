package acknowledgement

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

type additionalBlockBatch interface {
	SetAdditionalBlock(pos cube.Pos, runtimeID uint32)
	AdditionalBlocks() map[cube.Pos]uint32
}

func TestUpdateBlockBatchTracksAdditionalBlocks(t *testing.T) {
	ack := NewUpdateBlockBatchACK(nil)
	additional, ok := any(ack).(additionalBlockBatch)
	if !ok {
		t.Fatal("UpdateBlockBatch does not support additional blocks")
	}
	pos := cube.Pos{1, 2, 3}
	additional.SetAdditionalBlock(pos, 42)
	if got := additional.AdditionalBlocks()[pos]; got != 42 {
		t.Fatalf("additional runtime ID = %d, want 42", got)
	}
	if !ack.HasUpdates() {
		t.Fatal("batch with an additional block reports no updates")
	}
}
