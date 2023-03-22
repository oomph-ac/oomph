package player

import (
	"fmt"
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"reflect"
	"sync"
	"time"
)

// CachedChunk contains a pointer to the chunk, and the "subscriber" count.
// The subscriber count is the amount of players that are using the chunk.
type CachedChunk struct {
	sync.Mutex
	Chunk       *chunk.Chunk
	Subscribers uint32
}

// ChunkSubscriptionInfo contains the chunk position and the ID of the chunk.
// This is used to keep track of which chunks a player is subscribed to.
type ChunkSubscriptionInfo struct {
	ChunkPos       protocol.ChunkPos
	ID             int64
	GuaranteeTicks int64
}

var (
	chunkCacheMu       sync.Mutex
	chunkCache         map[protocol.ChunkPos]map[int64]*CachedChunk
	chunkSubscriptions map[int64]*ChunkSubscriptionInfo
	currentChunkID     int64 = 0
)

func init() {
	chunkCache = make(map[protocol.ChunkPos]map[int64]*CachedChunk)
	chunkSubscriptions = make(map[int64]*ChunkSubscriptionInfo)

	// Every 5 seconds, review the chunk cache, and remove any chunks that do not have any subscribers.
	go func() {
		t := time.NewTicker(time.Second * 2)
		for {
			select {
			case <-t.C:
				chunkCacheMu.Lock()
				deleted := 0
				for pos, subMap := range chunkCache {
					deleted += removeUnsubscribedChunks(subMap)

					// Remove duplicated chunks that have different IDs in a sub-map.
					if !removeDuplicateChunks(subMap) {
						continue
					}

					// removeDuplicateChunks returns true if the map
					// is empty, so we can delete it here.
					delete(chunkCache, pos)
				}

				// We run the garbage collector here to get rid of all the stupid
				// lurking chunks that are still in memory but are not used.
				// runtime.GC()

				chunkCacheMu.Unlock()
			}
		}
	}()
}

// compareChunkToMap compares a chunk to all chunks in a map. If the
// chunk is found in the map, it will return the ID and true.
func compareChunkToMap(c *chunk.Chunk, m map[int64]*CachedChunk) (int64, bool) {
	for id, chk := range m {
		if doChunkCompare(c, chk.Chunk) {
			return id, true
		}
	}

	return -1, false
}

// removeUnsubscribedChunks removes any chunks that have 0 subscribers
func removeUnsubscribedChunks(subMap map[int64]*CachedChunk) (d int) {
	for id, chk := range subMap {
		if chk.Subscribers > 0 {
			continue
		}

		d++
		delete(subMap, id)
		delete(chunkSubscriptions, id)
	}

	return
}

// removeDuplicateChunks removes duplicate chunks from the chunk cache.
// The function will return true if the map is empty.
func removeDuplicateChunks(subMap map[int64]*CachedChunk) bool {
	// Make a buffer of the ids of the chunks that have already been compared.
	// This is to prevent comparing two same chunks twice.
	compared := make(map[int64]bool, 0)

	for id1, chk := range subMap {
		for id2, chk2 := range subMap {
			// Don't compare the same chunk.
			if id1 == id2 {
				continue
			}

			// If the chunk has already been compared, skip it.
			if _, ok := compared[id2]; ok {
				continue
			}

			// Compare the chunks - if they are not the same, then they are not "duplicates".
			if !doChunkCompare(chk.Chunk, chk2.Chunk) {
				continue
			}

			// Edit the subscription info so that players using the duplicate chunk
			// that's going to get deleted will be subscribed to the chunk that's
			// going to stay.
			if i, ok := chunkSubscriptions[id2]; ok {
				i.ID = id1
			}

			delete(subMap, id2)
		}

		// Add the current chunk to the compared buffer.
		compared[id1] = true
	}

	compared = nil
	return len(subMap) == 0
}

// tryAddChunkToCache will try to add a chunk to the chunk cache. If the chunk is already exists
// in the cache, the function will return false.
func tryAddChunkToCache(p *Player, pos protocol.ChunkPos, c *chunk.Chunk) bool {
	if p == nil {
		panic("Did not expect null player when calling tryAddChunkToCache")
	}

	subMap, ok := chunkCache[pos]
	if !ok {
		// In this scenario, the map has not been created for this position yet, so we will create one.
		addChunkToCache(pos, c)
		p.subscribeToChunk(pos, currentChunkID)

		if p.debugger.Chunks {
			p.SendOomphDebug(fmt.Sprint("chunk ", pos, " had a sub-map created"), packet.TextTypeChat)
		}

		return true
	}

	// If the map exists, and the current chunk is found in the map, we will
	// have the player subscribe to that chunk.
	id, ok := compareChunkToMap(c, subMap)
	if ok {
		p.subscribeToChunk(pos, id)

		if p.debugger.Chunks {
			p.SendOomphDebug(fmt.Sprint("chunk ", pos, " was found in sub-map with id ", id), packet.TextTypeChat)
		}

		return false
	}

	// If the chunk is not found in the map, we will add it to the map.
	addChunkToCache(pos, c)
	p.subscribeToChunk(pos, currentChunkID)

	if p.debugger.Chunks {
		p.SendOomphDebug(fmt.Sprint("chunk ", pos, " added to cache in new sub-map"), packet.TextTypeChat)
	}

	return true
}

// getChunkFromCache returns a chunk from the chunk cache. If the chunk was found in the cache, it will return
// the chunk and true.
func getChunkFromCache(pos protocol.ChunkPos, id int64) (*CachedChunk, bool) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	// Check if there is a sub map for the chunk position.
	subMap, ok := chunkCache[pos]
	if !ok {
		// There is not a sub map for the chunk position, so we can return false.
		return nil, ok
	}

	chk, ok := subMap[id]
	if ok {
		// Lock the chunk before returning it - this is to ensure that once returned, we
		// can modify or read the chunk without any race conditions.
		chk.Lock()
		return chk, ok
	}

	// There was no chunk found in the sub map with the following ID, so we can return false.
	return nil, ok
}

// addChunkToCache adds a chunk to the chunk cache.
func addChunkToCache(pos protocol.ChunkPos, c *chunk.Chunk) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	// If the sub-map for the chunk position does not exist, we will create one.
	if _, ok := chunkCache[pos]; !ok {
		chunkCache[pos] = make(map[int64]*CachedChunk)
	}

	currentChunkID++

	chunkCache[pos][currentChunkID] = &CachedChunk{Chunk: c, Subscribers: 0}
	chunkSubscriptions[currentChunkID] = &ChunkSubscriptionInfo{ID: currentChunkID, ChunkPos: pos}
}

// doChunkCompare compares two chunks with each other. It will return true if
// the two given chunks are equal.
func doChunkCompare(c1 *chunk.Chunk, c2 *chunk.Chunk) bool {
	return reflect.DeepEqual(c1, c2)
}

// addChunkSubscribe will add a subscriber to a chunk in the chunk cache.
func addChunkSubscriber(pos protocol.ChunkPos, id int64) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	sub, ok := chunkCache[pos]
	if !ok {
		return
	}

	chk, ok := sub[id]
	if !ok {
		return
	}

	chk.Subscribers++
}

// getChunkSubscriptionInfo returns the chunk subscription info for the given chunk ID.
// This is done so that if we detect two identical chunks with different IDs, we can
// just edit the subscription info the chunk to auto-subscribe the player.
func getChunkSubscriptionInfo(id int64) *ChunkSubscriptionInfo {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	i, ok := chunkSubscriptions[id]
	if !ok {
		panic("Did not expect getting nil chunk subscription info")
	}

	return i
}

// removeChunkSubscriber will remove a subscriber from a chunk in the chunk cache.
func removeChunkSubscriber(pos protocol.ChunkPos, id int64) {
	chunkCacheMu.Lock()
	defer chunkCacheMu.Unlock()

	sub, ok := chunkCache[pos]
	if !ok {
		return
	}

	chk, ok := sub[id]
	if !ok {
		return
	}

	chk.Subscribers--
	if chk.Subscribers > 0 {
		return
	}

	delete(sub, id)
}

// GetChunkPos returns the chunk position from the given x and z coordinates.
func GetChunkPos(x, z int32) protocol.ChunkPos {
	return protocol.ChunkPos{x >> 4, z >> 4}
}

func (p *Player) subscribeToChunk(pos protocol.ChunkPos, id int64) {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	p.chunks[pos] = getChunkSubscriptionInfo(id)
	addChunkSubscriber(pos, id)
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
func (p *Player) Chunk(pos protocol.ChunkPos) (*CachedChunk, bool) {
	// Figure out of the player has a subscription to the chunk
	p.chkMu.Lock()
	defer p.chkMu.Unlock()
	sc, ok := p.chunks[pos]

	if !ok {
		return nil, false
	}

	// Check if the chunk is in the cache
	return getChunkFromCache(sc.ChunkPos, sc.ID)
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
	rid := c.Chunk.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)
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

	c.Chunk.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, rid)
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

// tickChunkSubscriptions lowers the guarantee ticks of all chunks that the player has a subscription to.
func (p *Player) tickChunkSubscriptions() {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	for _, sc := range p.chunks {
		if sc.GuaranteeTicks <= 0 {
			continue
		}

		sc.GuaranteeTicks--
	}
}

// makeChunkCopy makes a copy of the chunk at the given position, and
// adds it to the cache with a new ID.
func (p *Player) makeChunkCopy(pos protocol.ChunkPos) {
	p.chkMu.Lock()
	sc, ok := p.chunks[pos] // Check if the player has subscription info to the chunk at the given position.
	p.chkMu.Unlock()

	if !ok {
		// This function shouldn't be called anyway if the player doesn't have a
		// subscription to the chunk.
		return
	}

	// We can't make a copy right now - too fast! We can't just have the memory go brrrt...
	if sc.GuaranteeTicks > 0 {
		return
	}

	c, ok := getChunkFromCache(sc.ChunkPos, sc.ID)
	if !ok {
		// There should be a chunk here... am I missing something?
		return
	}

	// Remove the player from the unwanted original chunk.
	removeChunkSubscriber(sc.ChunkPos, sc.ID)

	c.Unlock()
	chk := *c.Chunk

	// Add the chunk to the cache with a new ID. This function also subscribes
	// the player to the copied chunk.
	tryAddChunkToCache(p, sc.ChunkPos, &chk)
	if nsc, ok := p.chunks[sc.ChunkPos]; ok {
		nsc.GuaranteeTicks = 40
	}
}

// cleanChunks filters out any chunks that are out of the player's view, and returns a value of
// how many chunks were cleaned
func (p *Player) cleanChunks() {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	loc := p.mInfo.ServerPosition
	activePos := world.ChunkPos{int32(math32.Floor(loc[0])) >> 4, int32(math32.Floor(loc[2])) >> 4}

	// Unsubscribe from any chunks that are out of the player's view.
	for _, sc := range p.chunks {
		diffX, diffZ := sc.ChunkPos[0]-activePos[0], sc.ChunkPos[1]-activePos[1]
		dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

		// If the distance is within the player's chunk view, leave it alone.
		if int32(dist) <= p.chunkRadius {
			continue
		}

		// The chunks are out of the player's view, so unsubscribe from them.
		removeChunkSubscriber(sc.ChunkPos, sc.ID)
		delete(p.chunks, sc.ChunkPos)
	}
}

// clearAllChunks clears all chunks from the player's chunk map and unsubscribes from any cached chunks.
func (p *Player) clearAllChunks() {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	for _, sc := range p.chunks {
		removeChunkSubscriber(sc.ChunkPos, sc.ID)
	}
	p.chunks = nil
}
