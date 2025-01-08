package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
)

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func unfuckDirection(dir int32) cube.Direction {
	dir -= 3
	if dir < 0 {
		dir = -dir
	}

	return cube.Direction(dir)
}

func fuckDirection(dir cube.Direction) int32 {
	newDir := int32(3 - dir)
	if newDir < 0 {
		newDir = -newDir
	}
	return newDir
}
