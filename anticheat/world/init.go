package world

import (
	"sync"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/anticheat/oerror"
	_ "github.com/oomph-ac/oomph/anticheat/world/block"
)

var (
	AirRuntimeID  uint32
	BlockRegistry = world.DefaultBlockRegistry
	initOnce      sync.Once
)

// FinalizeBlockRegistry finalizes the block registry and then caches the expected runtime ID for air.
func FinalizeBlockRegistry() {
	initOnce.Do(func() {
		BlockRegistry.Finalize()
		airRID, ok := BlockRegistry.StateToRuntimeID("minecraft:air", nil)
		if !ok {
			panic(oerror.New("unable to find runtime ID for air"))
		}
		AirRuntimeID = airRID
	})
}
