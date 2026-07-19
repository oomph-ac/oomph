package utils

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
)

func TestInitializeBlockNameMappingBeforeServerBootstrap(t *testing.T) {
	InitializeBlockNameMapping()
	if got := BlockName(block.Stone{}); got != "minecraft:stone" {
		t.Fatalf("BlockName(Stone{}) = %q, want minecraft:stone", got)
	}
}
