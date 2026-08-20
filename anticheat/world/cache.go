package world

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/world/blocknetwork"
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

func unsubC(hash xxh3.Uint128) {
	cMu.Lock()
	defer cMu.Unlock()

	if c, ok := chunkCache[hash]; ok {
		c.subs.Add(-1)
		if c.subs.Load() <= 0 {
			delete(chunkCache, hash)
		}
	}
}

func unsubSC(hash xxh3.Uint128) {
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

func CacheSubChunk(payload *bytes.Buffer, c *chunk.Chunk, pos protocol.ChunkPos, codec blocknetwork.Codec) (*CachedSubChunk, error) {
	scMu.Lock()
	defer scMu.Unlock()

	hash := xxh3.Hash128Seed(payload.Bytes(), uint64(codec.Mode()))
	if sc, ok := subChunkCache[hash]; ok {
		sc.subs.Add(1)
		//fmt.Println("returning cached subchunk", hash)
		return sc, nil
	}

	payloadLen := payload.Len()
	var index byte
	decodedSC, err := decodeSubChunk(payload, c, &index, chunk.NetworkEncoding)
	if err != nil {
		return nil, err
	}

	switch codec.Mode() {
	case blocknetwork.RuntimeIDs:
	case blocknetwork.Hashes:
		decodedSC.ConvertBlockNetworkHashesToRuntimeIDs(BlockRegistry)
	default:
		return nil, fmt.Errorf("unknown block network mode %d", codec.Mode())
	}

	cachedSC := &CachedSubChunk{hash: hash, layer: index, sc: decodedSC, payloadOffset: payloadLen - payload.Len()}
	cachedSC.subs.Add(1)
	subChunkCache[hash] = cachedSC

	//fmt.Println("newly cached subchunk", hash)
	return cachedSC, nil
}

func EncodeSubChunk(c *chunk.Chunk, index int, codec blocknetwork.Codec) ([]byte, error) {
	if index < 0 || index >= len(c.Sub()) {
		return nil, fmt.Errorf("invalid subchunk index %d", index)
	}

	switch codec.Mode() {
	case blocknetwork.RuntimeIDs:
		return chunk.EncodeSubChunk(c, chunk.NetworkEncoding, index), nil
	case blocknetwork.Hashes:
		return chunk.EncodeSubChunkWithBlockNetworkHashes(c, index), nil
	default:
		return nil, fmt.Errorf("unknown block network mode %d", codec.Mode())
	}
}

func CacheChunk(input *packet.LevelChunk, codec blocknetwork.Codec) (ChunkInfo, error) {
	cMu.Lock()
	defer cMu.Unlock()

	hash := xxh3.Hash128Seed(input.RawPayload, uint64(codec.Mode()))
	if c, ok := chunkCache[hash]; ok {
		c.subs.Add(1)
		//fmt.Println("returning cached chunk", hash)
		return ChunkInfo{Hash: hash, Chunk: c.chunk, Cached: true, PayloadOffset: c.payloadOffset}, nil
	}

	dimension, ok := world.DimensionByID(int(input.Dimension))
	if !ok {
		return ChunkInfo{}, fmt.Errorf("unknown dimension %v", input.Dimension)
	}

	buf := bytes.NewBuffer(input.RawPayload)
	decodedChunk, _, err := chunk.NetworkDecodeBuffer(
		BlockRegistry,
		buf,
		int(input.SubChunkCount),
		dimension.Range(),
	)
	if err != nil {
		return ChunkInfo{}, err
	}

	switch codec.Mode() {
	case blocknetwork.RuntimeIDs:
	case blocknetwork.Hashes:
		decodedChunk.ConvertBlockNetworkHashesToRuntimeIDs()
	default:
		return ChunkInfo{}, fmt.Errorf("unknown block network mode %d", codec.Mode())
	}
	decodedChunk.CompactForRuntimeCache()

	cachedChunk := &CachedChunk{hash: hash, chunk: decodedChunk, payloadOffset: len(input.RawPayload) - buf.Len()}
	cachedChunk.subs.Add(1)
	chunkCache[hash] = cachedChunk
	return ChunkInfo{Hash: hash, Chunk: cachedChunk.chunk, Cached: true, PayloadOffset: cachedChunk.payloadOffset}, nil
}

func EncodeLevelChunk(input *packet.LevelChunk, c *chunk.Chunk, payloadOffset int, codec blocknetwork.Codec) error {
	if payloadOffset < 0 || payloadOffset > len(input.RawPayload) {
		return fmt.Errorf("invalid level chunk payload offset %d", payloadOffset)
	}

	var data chunk.SerialisedData
	switch codec.Mode() {
	case blocknetwork.RuntimeIDs:
		data = chunk.Encode(c, chunk.NetworkEncoding)
	case blocknetwork.Hashes:
		data = chunk.EncodeWithBlockNetworkHashes(c)
	default:
		return fmt.Errorf("unknown block network mode %d", codec.Mode())
	}
	out := bytes.NewBuffer(make([]byte, 0, len(input.RawPayload)))
	for _, sub := range data.SubChunks {
		out.Write(sub)
	}
	out.Write(data.Biomes)
	out.Write(input.RawPayload[payloadOffset:])
	input.RawPayload = out.Bytes()
	input.SubChunkCount = uint32(len(data.SubChunks))
	return nil
}

// ReencodeLevelChunk fully re-encodes the block palettes in input while preserving the session's block-network
// representation and trailing block entity data.
func ReencodeLevelChunk(input *packet.LevelChunk, codec blocknetwork.Codec) error {
	dimension, ok := world.DimensionByID(int(input.Dimension))
	if !ok {
		return fmt.Errorf("unknown dimension %v", input.Dimension)
	}

	buf := bytes.NewBuffer(input.RawPayload)
	decoded, _, err := chunk.NetworkDecodeBuffer(BlockRegistry, buf, int(input.SubChunkCount), dimension.Range())
	if err != nil {
		return err
	}

	switch codec.Mode() {
	case blocknetwork.RuntimeIDs:
	case blocknetwork.Hashes:
		decoded.ConvertBlockNetworkHashesToRuntimeIDs()
	default:
		return fmt.Errorf("unknown block network mode %d", codec.Mode())
	}
	return EncodeLevelChunk(input, decoded, len(input.RawPayload)-buf.Len(), codec)
}

type CachedSubChunk struct {
	layer         byte
	hash          xxh3.Uint128
	subs          atomic.Int64
	sc            *chunk.SubChunk
	payloadOffset int
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

func (csc *CachedSubChunk) PayloadOffset() int {
	return csc.payloadOffset
}

type CachedChunk struct {
	hash          xxh3.Uint128
	subs          atomic.Int64
	chunk         *chunk.Chunk
	payloadOffset int
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
