package world

import (
	"sync"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

// World is the struct that contains chunks that are stored to create a "world".
type World struct {
	tick uint64

	chunks         map[protocol.ChunkPos]*chunk.Chunk
	exemptedChunks map[protocol.ChunkPos]uint8

	log *logrus.Logger
	mu  sync.Mutex
}

// NewWorld returns a new world.
func NewWorld(log *logrus.Logger) *World {
	return &World{
		log: log,

		chunks:         make(map[protocol.ChunkPos]*chunk.Chunk),
		exemptedChunks: make(map[protocol.ChunkPos]uint8),
	}
}

// Tick uns a tick on the world.
func (w *World) Tick() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Due to a bug of some sort, some chunks may be removed on
	for pos, t := range w.exemptedChunks {
		t--
		if t == 0 {
			delete(w.exemptedChunks, pos)
			w.log.Debugf("[WORLD @ %v] chunk %v no longer exempted", w.tick, pos)
			continue
		}

		w.exemptedChunks[pos] = t
	}

	w.tick++
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
	w.exemptedChunks[pos] = 200
	w.log.Debugf("[WORLD @ %v] chunk set at %v", w.tick, pos)
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

		// The chunk is currently exempted, so we should not remove it.
		if _, ok := w.exemptedChunks[p]; ok {
			continue
		}

		delete(w.chunks, p)
		w.log.Debugf("[WORLD @ %v] chunk removed at %v", w.tick, p)
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
	w.mu.Lock()
	defer w.mu.Unlock()

	maps.Clear(w.chunks)
	w.log.Debugf("[WORLD @ %v] chunks purged", w.tick)
}

// chunkInRange returns true if the chunk position is within the given radius of the chunk position.
func chunkInRange(radius int32, cpos, pos protocol.ChunkPos) bool {
	diffX, diffZ := pos[0]-cpos[0], pos[1]-cpos[1]
	dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

	return int32(dist) <= radius
}
