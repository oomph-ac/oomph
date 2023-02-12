package player

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"sync"
	"time"
)

type CachedChunk struct {
	Chunk       *chunk.Chunk
	Subscribers uint
}

var chunkCache map[protocol.ChunkPos]*CachedChunk
var chunkCacheMu sync.Mutex

func init() {
	chunkCache = make(map[protocol.ChunkPos]*CachedChunk)

	// Every 5 seconds, review the chunk cache, and remove any chunks that do not have any subscribers.
	go func() {
		t := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-t.C:
				chunkCacheMu.Lock()
				for pos, c := range chunkCache {
					if c.Subscribers == 0 {
						delete(chunkCache, pos)
					}
				}
				chunkCacheMu.Unlock()
			}
		}
	}()
}

// TryAddChunkToCache will try to add a chunk to the chunk cache. If the chunk is already in the cache, it will
// put it in the player's chunk map.
func TryAddChunkToCache(p *Player, pos protocol.ChunkPos, c *chunk.Chunk) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	chk, ok := chunkCache[pos]
	if !ok {
		chunkCache[pos] = &CachedChunk{Chunk: c}
		p.subscribeToChunk(pos)
		return
	}

	if chk.Chunk == c {
		p.subscribeToChunk(pos)
		return
	}

	p.LoadChunk(pos, c)
}

// GetChunkFromCache returns a chunk from the chunk cache. If the chunk was found in the cache, it will return
// the chunk and true.
func GetChunkFromCache(pos protocol.ChunkPos) (*chunk.Chunk, bool) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	c, ok := chunkCache[pos]
	if ok {
		c.Chunk.Lock()
		return c.Chunk, ok
	}

	return nil, ok
}

// CompareFromChunkCache returns true if the chunk in the chunk cache is the same as the chunk given.
func CompareFromChunkCache(pos protocol.ChunkPos, c *chunk.Chunk) (equal bool, exists bool) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	chk, ok := chunkCache[pos]
	if !ok {
		return false, false
	}

	return chk.Chunk == c, true
}

func addChunkSubscriber(pos protocol.ChunkPos) {
	chk, ok := chunkCache[pos]
	if !ok {
		return
	}

	chk.Subscribers++
}

func removeChunkSubscriber(pos protocol.ChunkPos) {
	chk, ok := chunkCache[pos]
	if !ok {
		return
	}

	chk.Subscribers--
}

func GetChunkPos(x, z int32) protocol.ChunkPos {
	return protocol.ChunkPos{x >> 4, z >> 4}
}

func (p *Player) subscribeToChunk(pos protocol.ChunkPos) {
	p.chkMu.Lock()
	p.subscribedChunks = append(p.subscribedChunks, pos)
	p.chkMu.Unlock()

	addChunkSubscriber(pos)
}

func (p *Player) LoadChunkFromCache(pos protocol.ChunkPos) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	if c, ok := chunkCache[pos]; ok {
		p.LoadChunk(pos, c.Chunk)
	}
}

// LoadChunk adds a chunk to the map of chunks
func (p *Player) LoadChunk(pos protocol.ChunkPos, c *chunk.Chunk) {
	p.chkMu.Lock()
	p.chunks[pos] = c
	p.chkMu.Unlock()
}

// UnloadChunk removes a chunk from the map of chunks
func (p *Player) UnloadChunk(pos protocol.ChunkPos) {
	p.chkMu.Lock()
	delete(p.chunks, pos)
	p.chkMu.Unlock()
}

// ChunkExists returns true if the given chunk position was found in the map of chunks
func (p *Player) ChunkExists(pos protocol.ChunkPos) bool {
	c, ok := p.Chunk(pos)
	if ok {
		c.Unlock()
	}

	return ok
}

// Chunk returns a chunk from the given chunk position. If the chunk was found in the map, it will
// return the chunk and true.
func (p *Player) Chunk(pos protocol.ChunkPos) (*chunk.Chunk, bool) {
	// First, we will check if the player has a different version of the chunk
	// loaded in their own chunk map.
	p.chkMu.Lock()
	c, ok := p.chunks[pos]
	p.chkMu.Unlock()

	if ok {
		c.Lock()
		return c, ok
	}

	// If there is no chunk detected, we will see if the chunk is in the cache.
	if c, ok := GetChunkFromCache(pos); ok {
		return c, ok
	}

	// No chunk detected - very sad :(
	return nil, ok
}

// tryToCacheChunks will try to remove chunks from the player's chunk map, and use the chunk cache instead.
// Chunks will only be removed if the chunk in the player's chunk map is the same as the chunk in the cache.
func (p *Player) tryToCacheChunks() {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	for pos, c := range p.chunks {
		// If the chunks differ from each other, we will not use the chunk cache for this chunk.
		if equal, _ := CompareFromChunkCache(pos, c); !equal {
			continue
		}

		// If the chunks are the same, we will remove the chunk from the player's chunk map, and use the cache.
		delete(p.chunks, pos)
	}
}

// Block returns the block found at the given position
func (p *Player) Block(pos cube.Pos) world.Block {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return block.Air{}
	}
	c, ok := p.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		return block.Air{}
	}
	rid := c.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)
	c.Unlock()

	b, _ := world.BlockByRuntimeID(rid)
	return b
}

// SetBlock sets a block at the given position to the given block
func (p *Player) SetBlock(pos cube.Pos, b world.Block) {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}

	rid := world.BlockRuntimeID(b)
	c, ok := p.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		return
	}

	c.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, rid)
	c.Unlock()
}

// GetNearbyBBoxes returns a list of block bounding boxes that are within the given bounding box - which is usually
// the player's bounding box.
func (p *Player) GetNearbyBBoxes(aabb cube.BBox) []cube.BBox {
	grown := aabb.Grow(1)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))

	// A prediction of one BBox per block, plus an additional 2, in case
	var bboxList []cube.BBox
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				boxes := p.Block(pos).Model().BBox(df_cube.Pos(pos), nil)
				for _, box := range boxes {
					b := game.DFBoxToCubeBox(box)
					if b.Translate(pos.Vec3()).IntersectsWith(aabb) {
						bboxList = append(bboxList, b.Translate(pos.Vec3()))
					}
				}
			}
		}
	}
	return bboxList
}

// GetNearbyBlocks returns a list of blocks that are within the given bounding box.
func (p *Player) GetNearbyBlocks(aabb cube.BBox) map[cube.Pos]world.Block {
	grown := aabb.Grow(0.25)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))

	// A prediction of one BBox per block, plus an additional 2, in case
	blocks := make(map[cube.Pos]world.Block)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				blocks[pos] = p.Block(pos)
			}
		}
	}

	return blocks
}

// cleanChunks filters out any chunks that are out of the player's view, and returns a value of
// how many chunks were cleaned
func (p *Player) cleanChunks() {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	loc := p.mInfo.ServerPosition
	activePos := world.ChunkPos{int32(math32.Floor(loc[0])) >> 4, int32(math32.Floor(loc[2])) >> 4}
	for pos := range p.chunks {
		diffX, diffZ := pos[0]-activePos[0], pos[1]-activePos[1]
		dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))
		if int(dist) > p.chunkRadius {
			delete(p.chunks, pos)
		}
	}

	subscribed := make([]protocol.ChunkPos, 0)
	for _, pos := range p.subscribedChunks {
		diffX, diffZ := pos[0]-activePos[0], pos[1]-activePos[1]
		dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))
		if int(dist) > p.chunkRadius {
			removeChunkSubscriber(pos)
			continue
		}

		subscribed = append(subscribed, pos)
	}
	p.subscribedChunks = subscribed
}

// clearAllChunks clears all chunks from the player's chunk map and unsubscribes from any cached chunks.
func (p *Player) clearAllChunks() {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	for _, pos := range p.subscribedChunks {
		removeChunkSubscriber(pos)
	}
	p.subscribedChunks = nil
	p.chunks = nil
}
