package collisions

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
)

var singleBBList = []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}

func ForBlock(b world.Block) []cube.BBox {
	name, properties := b.EncodeBlock()
	if bbs, ok := staticCollisions[name]; ok {
		return bbs
	}
	hash := hashBlockProperties(properties)
	if blockList, ok := collisionRegistry[name]; ok {
		if bbs, ok := blockList[hash]; ok {
			return bbs
		}
	}
	return singleBBList
}
