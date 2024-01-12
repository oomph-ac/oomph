package world

import (
	"runtime"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func init() {
	go handleQueue()
	go clearCacheDuplicates()
}

const (
	clearDuplicateDuration   = time.Second
	clearDuplicateGCDuration = time.Second * 5
)

var chunkCache = map[protocol.ChunkPos]map[uint64]*CachedChunk{}
var chunkIds = map[protocol.ChunkPos]uint64{}
var cacheMu sync.Mutex

func lazyInitCache(pos protocol.ChunkPos) {
	if chunkCache[pos] == nil {
		chunkCache[pos] = make(map[uint64]*CachedChunk)
		chunkIds[pos] = 0
	}
}

func clearCacheDuplicates() {
	lastGC := time.Now()
	for {
		time.Sleep(clearDuplicateDuration)

		cacheMu.Lock()
		for _, cacheMap := range chunkCache {
			for _, cached := range cacheMap {
				// If the cached chunk no longer has any subscribers, we can remove it from the cache.
				if len(cached.Subscribers) == 0 {
					delete(cacheMap, cached.ID)
				}

				// Check transaction list, and remove any unused or nil chunks.
				cached.Lock()
				for k, linkedC := range cached.Transactions {
					if linkedC == nil || len(linkedC.Subscribers) == 0 {
						delete(cached.Transactions, k)
					}
				}
				cached.Unlock()

				// Check for duplicate chunks in the cache.
				for _, other := range cacheMap {
					// If the IDs of the two chunks are not equal, but the chunks themselves are equal, we
					// can remove the other chunk from the cache and notify subscribers of the duplicated chunk
					// to use the original.
					if other.ID != cached.ID && other.Equals(cached.Chunk) {
						other.notifySubscriptionEdit(nil, cached)
					}
				}
			}
		}
		cacheMu.Unlock()

		// Run a garbage collection every 5 seconds, this is because memory cannot be freed
		// manually in Go, so we have to rely on the garbage collector to do it for us.
		if time.Since(lastGC) >= clearDuplicateGCDuration {
			runtime.GC()
			lastGC = time.Now()
		}
	}
}

var queuedChunks = make(chan *AddChunkRequest)

func handleQueue() {
	for {
		req, ok := <-queuedChunks
		if !ok {
			panic(oerror.New("chunk queue closed"))
		}

		// Search for a chunk in cache that is equal to the chunk in the request. If a matching
		// chunk is found, we add it to the world.
		matching := cacheSearchMatch(req.pos, req.c)
		if matching != nil {
			matching.Subscribe(req.w)
			continue
		}

		// Insert the chunk into the cache, and then add it to the world.
		cached := NewCached(req.pos, req.c)
		cached.Subscribe(req.w)
	}
}

func InsertToCache(w *World, c *chunk.Chunk, pos protocol.ChunkPos) {
	req := &AddChunkRequest{
		w:   w,
		c:   c,
		pos: pos,
	}

	select {
	case queuedChunks <- req:
		break
	case <-time.After(time.Second * 5):
		panic(oerror.New("chunk queue timed out"))
	}
}

type CachedChunk struct {
	*chunk.Chunk
	sync.Mutex

	ID  uint64
	Pos protocol.ChunkPos

	Transactions map[SetBlockAction]*CachedChunk
	Subscribers  map[uint64]*World
}

// NewCached creates and returns a new cached chunk.
func NewCached(pos protocol.ChunkPos, c *chunk.Chunk) *CachedChunk {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	lazyInitCache(pos)
	id := chunkIds[pos]
	chunkIds[pos]++

	cached := &CachedChunk{
		ID:           id,
		Chunk:        c,
		Pos:          pos,
		Transactions: make(map[SetBlockAction]*CachedChunk),
		Subscribers:  make(map[uint64]*World),
	}

	chunkCache[pos][id] = cached
	return cached
}

func (c *CachedChunk) Subscribe(w *World) {
	c.Lock()
	defer c.Unlock()

	w.AddChunk(c)
	c.Subscribers[w.id] = w
}

func (c *CachedChunk) Unsubscribe(w *World) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.Subscribers[w.id]; !ok {
		panic(oerror.New("cannot unsubscribe from chunk whilst not subscribed"))
	}

	w.RemoveChunk(c.Pos)
	delete(c.Subscribers, w.id)
}

func (c *CachedChunk) InsertSubChunk(w *World, sub *chunk.SubChunk, index byte) {
	c.Lock()
	defer c.Unlock()

	if len(c.Subscribers) == 1 {
		c.Sub()[index] = sub
		return
	}

	copiedChunk := *c.Chunk
	newCached := NewCached(c.Pos, &copiedChunk)
	newCached.Sub()[index] = sub

	c.notifySubscriptionEdit(w, newCached)
}

// ActionSetBlock sets the block in a chunk. The SetBlockAction contains the block position
// and the runtime ID of the block.
func (c *CachedChunk) ActionSetBlock(w *World, a SetBlockAction) {
	c.Lock()
	defer c.Unlock()

	// Verify that the action's block position is within range of the chunk.
	actionChunkPos := protocol.ChunkPos{int32(a.BlockPos[0]) >> 4, int32(a.BlockPos[2]) >> 4}
	if actionChunkPos != c.Pos {
		panic(oerror.New("action chunk pos does not match cached chunk pos"))
	}

	if new, ok := c.Transactions[a]; ok {
		c.notifySubscriptionEdit(w, new)
		return
	}

	// There is only one viewer of this chunk, so we can just update the chunk directly.
	if len(c.Subscribers) == 1 {
		c.SetBlock(uint8(a.BlockPos[0]), int16(a.BlockPos[1]), uint8(a.BlockPos[2]), 0, a.BlockRuntimeId)
		return
	}

	copiedChunk := *c.Chunk
	newCached := NewCached(c.Pos, &copiedChunk)
	newCached.SetBlock(uint8(a.BlockPos[0]), int16(a.BlockPos[1]), uint8(a.BlockPos[2]), 0, a.BlockRuntimeId)

	c.Transactions[a] = newCached
}

func (c *CachedChunk) notifySubscriptionEdit(w *World, new *CachedChunk) {
	// If the w is not nil, only one world needs their chunk updated, rather than all the subscribers
	// currently using the cached chunk.
	if w != nil {
		c.Unsubscribe(w)
		new.Subscribe(w)

		return
	}

	for _, sub := range c.Subscribers {
		c.Unsubscribe(sub)
		new.Subscribe(sub)
	}
}

func cacheSearchMatch(pos protocol.ChunkPos, c *chunk.Chunk) *CachedChunk {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	lazyInitCache(pos)
	for _, cached := range chunkCache[pos] {
		if cached.Equals(c) {
			return cached
		}
	}

	return nil
}
