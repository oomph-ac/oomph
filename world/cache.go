package world

import (
	"crypto/sha256"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

var (
	chunkCache = make(map[[32]byte]*CachedChunk)
	chunkQueue = internal.Chan[addChunkRequest](128, 1024*8)
	cMu        sync.RWMutex
)

func init() {
	for i := 0; i < runtime.NumCPU()-1; i++ {
		go cacheWorker()
	}
	go clearCacheWorker()
}

func Cache(w *World, input *packet.LevelChunk) (bool, error) {
	if chunkQueue.Send(addChunkRequest{input: input, target: w}, time.Second*5) {
		return true, nil
	}

	c, err := chunk.NetworkDecode(
		AirRuntimeID,
		input.RawPayload,
		int(input.SubChunkCount),
		world.Overworld.Range(),
	)
	if err != nil {
		c = chunk.New(AirRuntimeID, world.Overworld.Range())
	}
	c.Compact()
	w.AddChunk(input.Position, c)

	return false, err
}

type CachedChunk struct {
	subs atomic.Int64
	c    *chunk.Chunk
}

func (sc *CachedChunk) Subscribe() {
	sc.subs.Add(1)
}

func (sc *CachedChunk) Unsubscribe() {
	if newSubs := sc.subs.Add(-1); newSubs < 0 {
		panic(oerror.New("CachedChunk subscriber count below zero"))
	}
}

func (sc *CachedChunk) Block(x uint8, y int16, z uint8, layer uint8) (rid uint32) {
	return sc.c.Block(x, y, z, layer)
}

type addChunkRequest struct {
	input  *packet.LevelChunk
	target *World
}

func clearCacheWorker() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()

	for range t.C {
		cMu.Lock()
		for chunkHash, cachedChunk := range chunkCache {
			if cachedChunk.subs.Load() == 0 {
				delete(chunkCache, chunkHash)
			}
		}
		cMu.Unlock()
	}
}

func cacheWorker() {
	defer func() {
		hub := sentry.CurrentHub().Clone()
		if err := recover(); err != nil {
			hub.Recover(oerror.New("cacheWorker crashed: %v", err))
			hub.Flush(time.Second * 5)
		}
	}()

	for {
		req, ok := chunkQueue.Recv(nil)
		if !ok {
			break
		}

		// First, get the SHA-256 chunkHash of the packet's chunk payload.
		chunkHash := sha256.Sum256(req.input.RawPayload)
		// Then, check if the current chunk is already in the cache. If it is, add it to the player's world.
		if cached, found := findCachedChunk(chunkHash); found {
			cached.Subscribe()
			req.target.AddChunk(req.input.Position, cached)
		} else {
			// If the chunk is not in the cache, then we must decode the chunk accordingly.
			c, err := chunk.NetworkDecode(
				AirRuntimeID,
				req.input.RawPayload,
				int(req.input.SubChunkCount),
				world.Overworld.Range(), // TODO: decode chunks of other dimensions properly
			)
			if err != nil {
				c = chunk.New(AirRuntimeID, world.Overworld.Range())
			}
			c.Compact()

			// Create a new cached chunk and add a subscriber to it.
			cachedChunk := &CachedChunk{c: c}
			cachedChunk.subs.Store(1)

			// Add the new cached chunk to the cache.
			cMu.Lock()
			chunkCache[chunkHash] = cachedChunk
			cMu.Unlock()

			// Add the new cached chunk into the player's world.
			req.target.AddChunk(req.input.Position, cachedChunk)
		}
	}

	logrus.Warnf("cache worker shutdown")
}

// findCachedChunk returns a chunk at the given position that has the same hash as the one
func findCachedChunk(hash [32]byte) (*CachedChunk, bool) {
	cMu.RLock()
	cachedChunk, found := chunkCache[hash]
	cMu.RUnlock()

	return cachedChunk, found
}
