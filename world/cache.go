package world

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/zeebo/xxh3"
)

var (
	chunkCache = make(map[xxh3.Uint128]*CachedChunk)
	cMu        sync.Mutex

	subChunkCache = make(map[xxh3.Uint128]*CachedSubChunk)
	scMu          sync.Mutex
)

func unsubFromChunk(hash xxh3.Uint128) {
	cMu.Lock()
	defer cMu.Unlock()

	if c, ok := chunkCache[hash]; ok {
		c.subs.Add(-1)
		if c.subs.Load() <= 0 {
			delete(chunkCache, hash)
		}
	}
}

func unsubFromSubChunk(hash xxh3.Uint128) {
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

	hash := xxh3.Hash128(payload.Bytes())
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

func CacheChunk(input *packet.LevelChunk) (*CachedChunk, error) {
	cMu.Lock()
	defer cMu.Unlock()

	hash := xxh3.Hash128(input.RawPayload)
	if c, ok := chunkCache[hash]; ok {
		c.subs.Add(1)
		//fmt.Println("returning cached chunk", hash)
		return c, nil
	}

	dimension, ok := world.DimensionByID(int(input.Dimension))
	if !ok {
		return nil, fmt.Errorf("unknown dimension %v", input.Dimension)
	}

	buf := bytes.NewBuffer(input.RawPayload)
	decodedChunk, blobs, err := chunk.NetworkDecodeBuffer(
		AirRuntimeID,
		buf,
		int(input.SubChunkCount),
		dimension.Range(),
	)
	if err != nil {
		return nil, err
	}
	decodedChunk.Compact()

	cBlobs := make([]protocol.CacheBlob, len(blobs))
	for index, blob := range blobs {
		cBlobs[index] = protocol.CacheBlob{
			Payload: blob,
			Hash:    xxhash.Sum64(blob),
		}
	}

	cachedChunk := &CachedChunk{
		hash:   hash,
		chunk:  decodedChunk,
		blobs:  cBlobs,
		footer: buf.Bytes(),
	}
	_, _ = buf.ReadByte()
	cachedChunk.nbt = ChunkBlockEntities(decodedChunk, buf)
	cachedChunk.subs.Add(1)
	chunkCache[hash] = cachedChunk
	return cachedChunk, nil
}

type CachedSubChunk struct {
	layer byte
	hash  xxh3.Uint128
	subs  atomic.Int64
	sc    *chunk.SubChunk
}

func (csc *CachedSubChunk) Layer() byte {
	return csc.layer
}

func (csc *CachedSubChunk) Hash() xxh3.Uint128 {
	return csc.hash
}

func (csc *CachedSubChunk) SubChunk() *chunk.SubChunk {
	return csc.sc
}

type CachedChunk struct {
	hash  xxh3.Uint128
	subs  atomic.Int64
	chunk *chunk.Chunk

	nbt map[df_cube.Pos]world.Block

	blobs  []protocol.CacheBlob
	footer []byte
}

// Chunk returns a dereferenced copy of the chunk stored.
func (cc *CachedChunk) Chunk() *chunk.Chunk {
	return cc.chunk
}

func (cc *CachedChunk) Hash() xxh3.Uint128 {
	return cc.hash
}

func (cc *CachedChunk) Block(x uint8, y int16, z uint8, layer uint8) (rid uint32) {
	return cc.chunk.Block(x, y, z, layer)
}

func (cc *CachedChunk) BlockNBT(pos df_cube.Pos) (world.Block, bool) {
	if b, ok := cc.nbt[pos]; ok {
		return b, true
	}
	return nil, false
}

func (cc *CachedChunk) Blobs() []protocol.CacheBlob {
	return cc.blobs
}

func (cc *CachedChunk) Footer() []byte {
	return cc.footer
}
