package utils

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
)

func TestBlockName(t *testing.T) {
	if got := BlockName(block.Stone{}); got != "minecraft:stone" {
		t.Fatalf("BlockName(Stone{}) = %q, want minecraft:stone", got)
	}
}
