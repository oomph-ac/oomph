package world

import (
	"sync"

	_ "unsafe"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/oerror"
	_ "github.com/oomph-ac/oomph/world/block"
)

var (
	AirRuntimeID uint32
	initOnce     sync.Once
)

// noinspection ALL
//
//go:linkname world_finaliseBlockRegistry github.com/df-mc/dragonfly/server/world.finaliseBlockRegistry
func world_finaliseBlockRegistry()

// FinalizeBlockRegistry finalizes the block registry and then caches the expected runtime ID for air.
func FinalizeBlockRegistry() {
	initOnce.Do(func() {
		world_finaliseBlockRegistry()
		airRID, ok := chunk.StateToRuntimeID("minecraft:air", nil)
		if !ok {
			panic(oerror.New("unable to find runtime ID for air"))
		}
		AirRuntimeID = airRID
	})
}
