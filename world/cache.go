package world

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"sync/atomic"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var (
	chunkCache = make(map[[32]byte]*CachedChunk)
	cMu        sync.Mutex

	subChunkCache = make(map[[32]byte]*CachedSubChunk)
	scMu          sync.Mutex
)

func unsubC(hash [32]byte) {
	cMu.Lock()
	defer cMu.Unlock()

	if c, ok := chunkCache[hash]; ok {
		c.subs.Add(-1)
		if c.subs.Load() <= 0 {
			delete(chunkCache, hash)
		}
	}
}

func unsubSC(hash [32]byte) {
	scMu.Lock()
	defer scMu.Unlock()

	if c, ok := subChunkCache[hash]; ok {
		//fmt.Println("unsubscribing from subchunk", hash, c.subs.Load())
		c.subs.Add(-1)
		if c.subs.Load() <= 0 {
			//fmt.Println("deleting subchunk from cache", hash)
			delete(subChunkCache, hash)
		}
	}
}

func CacheSubChunk(payload *bytes.Buffer, c *chunk.Chunk, pos protocol.ChunkPos) (*CachedSubChunk, error) {
	scMu.Lock()
	defer scMu.Unlock()

	hash := sha256.Sum256(payload.Bytes())
	if sc, ok := subChunkCache[hash]; ok {
		sc.subs.Add(1)
		//fmt.Println("returning cached subchunk", hash)
		return sc, nil
	}

	var index byte
	decodedSC, err := decodeSubChunk(payload, c, &index, chunk.NetworkEncoding)
	if err != nil {
		return nil, err
	}

	cachedSC := &CachedSubChunk{hash: hash, layer: index, sc: decodedSC}
	cachedSC.subs.Add(1)
	subChunkCache[hash] = cachedSC

	//fmt.Println("newly cached subchunk", hash)
	return cachedSC, nil
}

func CacheChunk(input *packet.LevelChunk) ChunkInfo {
	cMu.Lock()
	defer cMu.Unlock()

	hash := sha256.Sum256(input.RawPayload)
	if c, ok := chunkCache[hash]; ok {
		c.subs.Add(1)
		//fmt.Println("returning cached chunk", hash)
		return ChunkInfo{Hash: hash, Chunk: c.chunk, Cached: true}
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

	cachedChunk := &CachedChunk{hash: hash, chunk: decodedChunk}
	cachedChunk.subs.Add(1)
	chunkCache[hash] = cachedChunk
	return ChunkInfo{Hash: hash, Chunk: cachedChunk.chunk, Cached: true}
}

type CachedSubChunk struct {
	layer byte
	hash  [32]byte
	subs  atomic.Int64
	sc    *chunk.SubChunk
}

func (csc *CachedSubChunk) Layer() byte {
	return csc.layer
}

func (csc *CachedSubChunk) Hash() [32]byte {
	return csc.hash
}

func (csc *CachedSubChunk) SubChunk() *chunk.SubChunk {
	return csc.sc
}

type CachedChunk struct {
	hash  [32]byte
	subs  atomic.Int64
	chunk *chunk.Chunk
}

// Chunk returns a dereferenced copy of the chunk stored.
func (cc *CachedChunk) Chunk() *chunk.Chunk {
	return cc.chunk
}

func (cc *CachedChunk) Hash() [32]byte {
	return cc.hash
}

func (cc *CachedChunk) Block(x uint8, y int16, z uint8, layer uint8) (rid uint32) {
	return cc.chunk.Block(x, y, z, layer)
}
