package world

import (
	"sync"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"golang.org/x/exp/maps"
)

// World is the struct that contains chunks that are stored to create a "world".
type World struct {
	chunks map[protocol.ChunkPos]*chunk.Chunk
	mu     sync.Mutex
}

// NewWorld returns a new world.
func NewWorld() *World {
	return &World{
		chunks: make(map[protocol.ChunkPos]*chunk.Chunk),
	}
}

// ChunkExists returns true if the chunk at the given position exists.
func (w *World) ChunkExists(pos protocol.ChunkPos) bool {
	_, ok := w.chunks[pos]
	return ok
}

// Chunk returns the chunk at the given position. If the chunk does not exist, nil is returned.
func (w *World) Chunk(pos protocol.ChunkPos) *chunk.Chunk {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.chunks[pos]
}

// SetChunk sets the chunk at the given position to the chunk passed.
func (w *World) SetChunk(c *chunk.Chunk, pos protocol.ChunkPos) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.chunks[pos] = c
}

// GetBlock returns the block at the given position.
func (w *World) GetBlock(pos cube.Pos) world.Block {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return block.Air{}
	}

	c := w.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if c == nil {
		return block.Air{}
	}

	rid := c.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)
	b, ok := world.BlockByRuntimeID(rid)

	if !ok {
		return block.Air{}
	}
	return b
}

// SetBlock sets the block at the given position to the block passed.
func (w *World) SetBlock(pos cube.Pos, b world.Block) {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}

	rid := world.BlockRuntimeID(b)
	c := w.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if c == nil {
		return
	}

	c.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, rid)
}

// CleanChunks cleans up the chunks in respect to the given chunk radius and chunk position.
func (w *World) CleanChunks(r int32, pos protocol.ChunkPos) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for p := range w.chunks {
		if chunkInRange(r, p, pos) {
			continue
		}

		delete(w.chunks, p)
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
	w.mu.Lock()
	defer w.mu.Unlock()

	maps.Clear(w.chunks)
}

// chunkInRange returns true if the chunk position is within the given radius of the chunk position.
func chunkInRange(radius int32, cpos, pos protocol.ChunkPos) bool {
	diffX, diffZ := pos[0]-cpos[0], pos[1]-cpos[1]
	dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

	return int32(dist) <= radius
}
