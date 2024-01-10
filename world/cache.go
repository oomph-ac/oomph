package world

import (
	"runtime"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	cacheRemoveDupeDuration = time.Millisecond * 50
	cacheCleanDuration      = time.Second * 5
)

var chunkCache = map[protocol.ChunkPos]map[uint64]*CachedChunk{}
var chunkIDList = map[protocol.ChunkPos]uint64{}
var cacheMu sync.Mutex

func init() {
	go cleanChunkCache()
}

func lazyInitCache(pos protocol.ChunkPos) map[uint64]*CachedChunk {
	if _, ok := chunkCache[pos]; !ok {
		chunkCache[pos] = map[uint64]*CachedChunk{}
		chunkIDList[pos] = 0
	}

	return chunkCache[pos]
}

func cleanChunkCache() {
	lastGC := time.Now()
	for {
		time.Sleep(cacheRemoveDupeDuration)

		cacheMu.Lock()
		for chunkPos, cacheMap := range chunkCache {
			for id, cached := range cacheMap {
				if cached == nil || len(cached.Subscribers) == 0 {
					delete(cacheMap, id)
					continue
				}

				for otherID, otherCached := range cacheMap {
					if otherID != id && cached.Chunk.Equals(otherCached.Chunk) {
						otherCached.notifyUpdate(chunkPos, cached, nil)
						delete(cacheMap, otherID)
					}
				}
			}
		}
		cacheMu.Unlock()

		// Let the garbage collector get rid of chunks that are no longer used.
		// TODO: Determine if having GC() run here is beneficial.
		if time.Since(lastGC) >= cacheCleanDuration {
			runtime.GC()
			lastGC = time.Now()
		}
	}
}

// CachedChunk is a struct that represent a chunk in the cache. It contains the chunk itself, along with
// an ID used to identify the version of the chunk, and the subscribers using the chunk.
type CachedChunk struct {
	*chunk.Chunk
	ID uint64

	// Transactions is a map of transactions that have been made to the chunk, along
	// with the ID of the new chunk that was created as a result of the transaction.
	Transactions map[SetBlockTransaction]uint64
	Subscribers  map[uint64]*World
}

func NewCachedChunk(pos protocol.ChunkPos, c *chunk.Chunk) *CachedChunk {
	cached := &CachedChunk{
		Chunk: c,

		ID:           chunkIDList[pos],
		Subscribers:  map[uint64]*World{},
		Transactions: map[SetBlockTransaction]uint64{},
	}
	chunkIDList[pos]++

	return cached
}

func (c *CachedChunk) Subscribe(w *World) {
	c.Subscribers[w.id] = w
}

func (c *CachedChunk) Unsubscribe(w *World) {
	delete(c.Subscribers, w.id)
}

// Execute runs a set block transaction.
func (c *CachedChunk) Execute(w *World, pos protocol.ChunkPos, tx SetBlockTransaction) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// If the transaction has already been made, find the cached chunk and subscribe the world to it.
	if newID, ok := c.Transactions[tx]; ok {
		newC, ok := chunkCache[pos][newID]

		// It is possible that the chunk has been removed from the cache because it was a duplicate.
		if ok {
			c.notifyUpdate(pos, newC, w)
		}
	}

	// We don't need to create a copy of the chunk if there is only one subscriber, and instead, we can
	// just directly update the chunk w/ the block update.
	if len(c.Subscribers) == 1 {
		tx.Execute(c.Chunk)
		return
	}

	copiedChunk := *c.Chunk
	newCached := NewCachedChunk(pos, &copiedChunk)

	// Notify the world of the update to the chunk
	c.notifyUpdate(pos, newCached, w)
	tx.Execute(newCached.Chunk)
	c.Transactions[tx] = newCached.ID

	// Set the new chunk into the cache.
	chunkCache[pos][newCached.ID] = newCached
}

// notifyUpdate notifies the subscribers of the chunk that the chunk has been updated. The chunk passed
// is the new chunk that the subscribers should be subscribed to. If wld is nil, all subscribers are
// unsubscribed from the old chunk.
func (c *CachedChunk) notifyUpdate(pos protocol.ChunkPos, new *CachedChunk, wld *World) {
	// If world is not nil here, we are assuming the mutex of the world is already locked, and
	// therefore do not lock it here to prevent a deadlock.
	if wld != nil {
		// Unsubscribe the old chunk (us), and subscribe to the new chunk.
		c.Unsubscribe(wld)
		new.Subscribe(wld)
		wld.chunks[pos] = new.ID
	}

	for _, w := range c.Subscribers {
		w.Lock()

		// Unsubscribe the old chunk (us), and subscribe to the new chunk.
		c.Unsubscribe(w)
		w.chunks[pos] = new.ID
		new.Subscribe(w)

		w.Unlock()
	}
}

// InsertIntoCache inserts a chunk into the cache.
func InsertIntoCache(w *World, pos protocol.ChunkPos, c *chunk.Chunk) uint64 {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	lazyInitCache(pos)
	matching := cacheSearchMatching(pos, c)

	if matching != nil {
		matching.Subscribe(w)
		return matching.ID
	}

	cache := NewCachedChunk(pos, c)
	cache.Subscribe(w)
	chunkCache[pos][cache.ID] = cache

	return cache.ID
}

// SearchFromCache searches for a chunk in the cache.
func SearchFromCache(pos protocol.ChunkPos, id uint64) *CachedChunk {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	cacheMap := lazyInitCache(pos)
	return cacheMap[id]
}

// cacheSearchMatching searches for a match in the cache. If the chunk is not found in the
// cache, nil is returned.
func cacheSearchMatching(pos protocol.ChunkPos, c *chunk.Chunk) *CachedChunk {
	cacheMap := lazyInitCache(pos)
	for _, cached := range cacheMap {
		if cached.Equals(c) {
			return cached
		}
	}

	return nil
}
