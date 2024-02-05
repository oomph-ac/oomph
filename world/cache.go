package world

import (
	"runtime"
	"time"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sasha-s/go-deadlock"
)

func init() {
	go clearCacheDuplicates()
}

const (
	clearDuplicateDuration   = time.Second
	clearDuplicateGCDuration = time.Second * 5
)

var chunkCache = map[protocol.ChunkPos]map[uint64]*CachedChunk{}
var chunkIds = map[protocol.ChunkPos]uint64{}
var cacheMu deadlock.Mutex

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
					continue
				}

				// Check transaction list, and remove any unused or nil chunks.
				cached.txMu.Lock()
				for k, linkedC := range cached.Transactions {
					if linkedC == nil || len(linkedC.Subscribers) == 0 {
						delete(cached.Transactions, k)
						continue
					}

					found, ok := chunkCache[linkedC.Pos][linkedC.ID]
					if !ok || !found.Equals(linkedC.Chunk) {
						delete(cached.Transactions, k)
					}
				}
				cached.txMu.Unlock()

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

func InsertToCache(w *World, c *chunk.Chunk, pos protocol.ChunkPos) {
	req := &AddChunkRequest{
		w:   w,
		c:   c,
		pos: pos,
	}

	select {
	case queuedChunks <- req:
		break
	case <-time.After(time.Second * 10):
		panic(oerror.New("chunk queue timed out"))
	}
}

type CachedChunk struct {
	*chunk.Chunk
	deadlock.RWMutex

	ID  uint64
	Pos protocol.ChunkPos

	Transactions map[SetBlockAction]*CachedChunk
	txMu         deadlock.RWMutex

	Subscribers map[uint64]*World
	sMu         deadlock.RWMutex
}

// NewChunk creates and runs a callable function on a new CachedChunk. The function is called with the
// new CachedChunk as an argument, allowing for the chunk to be modified before it is added to the cache.
func NewChunk(pos protocol.ChunkPos, c *chunk.Chunk, modFunc func(*CachedChunk)) {
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

	modFunc(cached)
	chunkCache[pos][id] = cached
}

func (c *CachedChunk) Subscribe(w *World) {
	c.sMu.Lock()
	defer c.sMu.Unlock()

	w.AddChunk(c)
	c.Subscribers[w.id] = w
}

func (c *CachedChunk) Unsubscribe(w *World) {
	c.sMu.Lock()
	defer c.sMu.Unlock()

	w.RemoveChunk(c.Pos)
	delete(c.Subscribers, w.id)
}

func (c *CachedChunk) InsertSubChunk(w *World, sub *chunk.SubChunk, index byte) {
	if len(c.Subscribers) == 1 {
		c.Lock()
		c.Sub()[index] = sub
		c.Unlock()

		return
	}

	// Check if the sub chunk is equal, and is equivilent - no need to do anything.
	if int(index) < len(c.Sub()) && c.Sub()[index].Equals(sub) {
		return
	}

	copiedChunk := *c.Chunk
	NewChunk(c.Pos, &copiedChunk, func(new *CachedChunk) {
		new.Sub()[index] = sub
		c.notifySubscriptionEdit(w, new)
	})
}

// ActionSetBlock sets the block in a chunk. The SetBlockAction contains the block position
// and the runtime ID of the block.
func (c *CachedChunk) ActionSetBlock(w *World, a SetBlockAction) {
	// Verify that the action's block position is within range of the chunk.
	actionChunkPos := protocol.ChunkPos{int32(a.BlockPos[0]) >> 4, int32(a.BlockPos[2]) >> 4}
	if actionChunkPos != c.Pos {
		panic(oerror.New("action chunk pos does not match cached chunk pos"))
	}

	c.txMu.RLock()
	new, ok := c.Transactions[a]
	c.txMu.RUnlock()

	if ok {
		c.notifySubscriptionEdit(w, new)
		return
	}

	// There is only one viewer of this chunk, so we can just update the chunk directly.
	if len(c.Subscribers) == 1 {
		c.Lock()
		c.SetBlock(uint8(a.BlockPos[0]), int16(a.BlockPos[1]), uint8(a.BlockPos[2]), 0, a.BlockRuntimeId)
		c.Unlock()

		return
	}

	c.Lock()
	copiedChunk := *c.Chunk
	c.Unlock()

	NewChunk(c.Pos, &copiedChunk, func(new *CachedChunk) {
		new.SetBlock(uint8(a.BlockPos[0]), int16(a.BlockPos[1]), uint8(a.BlockPos[2]), 0, a.BlockRuntimeId)
		c.notifySubscriptionEdit(w, new)

		c.txMu.Lock()
		c.Transactions[a] = new
		c.txMu.Unlock()
	})
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
