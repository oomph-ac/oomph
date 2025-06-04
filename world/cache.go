package world

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var (
	chunkCache = make(map[[32]byte]*CachedChunk)
	cMu        sync.Mutex
)

func Cache(input *packet.LevelChunk) (*CachedChunk, error) {
	cMu.Lock()
	defer cMu.Unlock()

	hash := sha256.Sum256(input.RawPayload)
	if c, ok := chunkCache[hash]; ok {
		c.Subscribe()
		return c, nil
	}

	decodedChunk, err := chunk.NetworkDecode(
		AirRuntimeID,
		input.RawPayload,
		int(input.SubChunkCount),
		world.Overworld.Range(),
	)
	if err != nil {
		decodedChunk = chunk.New(AirRuntimeID, world.Overworld.Range())
	}
	decodedChunk.Compact()
	c := &CachedChunk{hash: hash, chunk: decodedChunk}
	c.Subscribe()
	chunkCache[hash] = c
	return c, err
}

type CachedChunk struct {
	hash  [32]byte
	subs  atomic.Int64
	chunk *chunk.Chunk
}

// Chunk returns a dereferenced copy of the chunk stored.
func (sc *CachedChunk) Chunk() chunk.Chunk {
	return *sc.chunk
}

func (sc *CachedChunk) Hash() string {
	return fmt.Sprintf("%x", sc.hash)
}

func (sc *CachedChunk) Subscribe() {
	sc.subs.Add(1)
}

func (sc *CachedChunk) Unsubscribe() {
	if sc.subs.Add(-1) <= 0 {
		cMu.Lock()
		delete(chunkCache, sc.hash)
		cMu.Unlock()
	}
}

func (sc *CachedChunk) Block(x uint8, y int16, z uint8, layer uint8) (rid uint32) {
	return sc.chunk.Block(x, y, z, layer)
}
