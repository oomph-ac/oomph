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
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

var (
	chunkCache = make(map[[32]byte]*CachedChunk)
	chunkQueue = make(chan addChunkRequest, 65536)
	cMu        sync.RWMutex
)

func init() {
	for i := 0; i < runtime.NumCPU()*2; i++ {
		go cacheWorker()
	}
	go clearCacheWorker()
}

func Cache(w *World, input *packet.LevelChunk) (bool, error) {
	select {
	case chunkQueue <- addChunkRequest{input: input, target: w}:
		return true, nil
	default:
		// In this case, the chunk request channel is already filled up (wow) which means the workers are a bit behind.
		// We can still process the chunk manually and insert it into the player's world.
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
}

type CachedChunk struct {
	subs atomic.Int64
	c    *chunk.Chunk
}

func (sc *CachedChunk) Subscribe() {
	sc.subs.Add(1)
}

func (sc *CachedChunk) Unsubscribe() {
	sc.subs.Add(-1)
}

func (sc *CachedChunk) Block(x uint8, y int16, z uint8, layer uint8) (rid uint32) {
	return sc.c.Block(x, y, z, layer)
}

type addChunkRequest struct {
	input  *packet.LevelChunk
	target *World
}

func clearCacheWorker() {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	defer func() {
		if err := recover(); err != nil {
			hub := sentry.CurrentHub().Clone()
			hub.Recover(err)
			hub.Flush(time.Second * 5)
		}
	}()

	for range t.C {
		cMu.Lock()
		for chunkHash, cachedChunk := range chunkCache {
			if cachedChunk.subs.Load() <= 0 {
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
		req, ok := <-chunkQueue
		if !ok {
			break
		}

		// First, get the SHA-256 chunkHash of the packet's chunk payload.
		chunkHash := sha256.Sum256(req.input.RawPayload)

		// We can't just do a read-lock on the cache mutex because there could be a race condition where
		// the chunk is not found and then in another worker, a chunk with the same hash is cached.
		cMu.Lock()
		// Lookup the chunk in the cache.
		cachedChunk, found := chunkCache[chunkHash]
		// If the chunk is not found in the cache, we can create it and insert it into the cache.
		if !found {
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

			cachedChunk = &CachedChunk{c: c}
			chunkCache[chunkHash] = cachedChunk
		}
		// Subscribe to the cached chunk and add it into the player's world.
		cachedChunk.Subscribe()
		req.target.AddChunk(req.input.Position, cachedChunk)
		cMu.Unlock()
	}

	logrus.Warnf("cache worker shutdown")
}
