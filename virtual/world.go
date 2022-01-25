package virtual

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sirupsen/logrus"
	"math"
	"sync"
)

// World keeps track of all world specific data, such as chunks.
type World struct {
	log *logrus.Logger

	chunkMu   sync.Mutex
	chunks    map[world.ChunkPos]*chunk.Chunk
	dimension world.Dimension
}

// NewWorld creates a new world.
func NewWorld(log *logrus.Logger, dimension world.Dimension) *World {
	return &World{
		log:       log,
		chunks:    make(map[world.ChunkPos]*chunk.Chunk),
		dimension: dimension,
	}
}

// OutOfBounds returns true if the chunk is out of a view distance bounds.
func (w *World) OutOfBounds(pos, activePos world.ChunkPos, viewDistance int32) bool {
	diffX, diffZ := pos[0]-activePos[0], pos[1]-activePos[1]
	dist := math.Sqrt(float64(diffX*diffX) + float64(diffZ*diffZ))
	if int32(dist) > viewDistance {
		return true
	}
	return false
}

// LoadRawChunk loads a chunk from raw data.
func (w *World) LoadRawChunk(pos world.ChunkPos, data []byte, subChunkCount uint32) {
	ch, err := chunk.NetworkDecode(w.air(), data, int(subChunkCount), w.dimension.Range())
	if err != nil {
		w.log.Errorf("failed to parse chunk at %v: %v", pos, err)
		return
	}
	ch.Compact()
	w.LoadChunk(pos, ch)
}

// LoadChunk loads a chunk at a position in the world.
func (w *World) LoadChunk(pos world.ChunkPos, c *chunk.Chunk) {
	w.chunkMu.Lock()
	w.chunks[pos] = c
	w.chunkMu.Unlock()
}

// UnloadChunk unloads a chunk at a position in the world.
func (w *World) UnloadChunk(pos world.ChunkPos) {
	w.chunkMu.Lock()
	delete(w.chunks, pos)
	w.chunkMu.Unlock()
}

// Chunk attempts to return a chunk in the world. If it does not exist, the second return value will be false.
func (w *World) Chunk(pos world.ChunkPos) (*chunk.Chunk, bool) {
	w.chunkMu.Lock()
	c, ok := w.chunks[pos]
	c.Lock()
	w.chunkMu.Unlock()
	return c, ok
}

// Block reads a block from the position passed. If a chunk is not yet loaded at that position, the chunk is
// loaded, or generated if it could not be found in the world save, and the block returned. Chunks will be
// loaded synchronously.
func (w *World) Block(pos cube.Pos) world.Block {
	if pos.OutOfBounds(w.dimension.Range()) {
		return block.Air{}
	}
	c, ok := w.Chunk(world.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		w.log.Errorf("failed to query chunk at %v", pos)
		return block.Air{}
	}
	rid := c.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)
	c.Unlock()

	b, _ := world.BlockByRuntimeID(rid)
	return b
}

// SetBlock writes a block to the position passed. If a chunk is not yet loaded at that position, the chunk is
// first loaded or generated if it could not be found in the world save.
// SetBlock panics if the block passed has not yet been registered using RegisterBlock().
// Nil may be passed as the block to set the block to air.
// SetBlock should be avoided in situations where performance is critical when needing to set a lot of blocks
// to the world. BuildStructure may be used instead.
func (w *World) SetBlock(pos cube.Pos, b world.Block) {
	if w == nil || pos.OutOfBounds(w.dimension.Range()) {
		return
	}

	rid, ok := world.BlockRuntimeID(b)
	if !ok {
		w.log.Errorf("failed to query runtime id for %v at %v", rid, pos)
		return
	}

	c, ok := w.Chunk(world.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		w.log.Errorf("failed to query chunk at %v", pos)
		return
	}
	c.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, rid)
	c.Unlock()
}

// air returns the air runtime ID.
func (w *World) air() uint32 {
	air, _ := chunk.StateToRuntimeID("minecraft:air", nil)
	return air
}
