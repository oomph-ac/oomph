package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func fuckDirection(dir cube.Direction) int32 {
	newDir := int32(3 - dir)
	if newDir < 0 {
		newDir = -newDir
	}
	return newDir
}

func registerBlock(b world.Block) {
	defer func() {
		if recover() != nil {
			// Some supplemental blocks have moved into Dragonfly. Keep the rest registered without failing on those.
		}
	}()
	world.RegisterBlock(b)
}
