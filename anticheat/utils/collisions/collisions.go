package collisions

import (
	"math"
	"sync"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
)

var (
	singleBBList = []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	// Unknown blocks all use the same sentinel hash, so caching by hash would mix them up.
	// Skip the cache and use the slow path instead.
	forBlockCache sync.Map
)

func ForBlock(b world.Block) []cube.BBox {
	if _, h := b.Hash(); h == math.MaxUint64 {
		return forBlockSlow(b)
	}
	key := world.BlockHash(b)
	if v, ok := forBlockCache.Load(key); ok {
		return v.([]cube.BBox)
	}
	bbs := forBlockSlow(b)
	forBlockCache.Store(key, bbs)
	return bbs
}

func forBlockSlow(b world.Block) []cube.BBox {
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
