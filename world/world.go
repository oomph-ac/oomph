package world

import (
	"log/slog"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sasha-s/go-deadlock"

	_ "unsafe"

	_ "github.com/oomph-ac/oomph/world/block"
)

var currentWorldId uint64

type World struct {
	id           uint64
	lastCleanPos protocol.ChunkPos

	chunks         map[protocol.ChunkPos]ChunkSource
	exemptedChunks map[protocol.ChunkPos]struct{}
	blockUpdates   map[protocol.ChunkPos]map[df_cube.Pos]world.Block

	logger **slog.Logger

	deadlock.RWMutex
}

func New(logger **slog.Logger) *World {
	currentWorldId++
	return &World{
		chunks:         make(map[protocol.ChunkPos]ChunkSource),
		exemptedChunks: make(map[protocol.ChunkPos]struct{}),
		blockUpdates:   make(map[protocol.ChunkPos]map[df_cube.Pos]world.Block),
		id:             currentWorldId,
		logger:         logger,
	}
}

// AddChunk adds a chunk to the world.
func (w *World) AddChunk(chunkPos protocol.ChunkPos, c ChunkSource) {
	w.Lock()
	defer w.Unlock()

	if old, ok := w.chunks[chunkPos]; ok {
		if cached, ok := old.(*CachedChunk); ok {
			cached.Unsubscribe()
		}
		delete(w.blockUpdates, chunkPos)
	}
	w.chunks[chunkPos] = c
	w.exemptedChunks[chunkPos] = struct{}{}
}

// GetChunk returns a cached chunk at the position passed. The mutex is
// not locked here because it is assumed that the caller has already locked
// the mutex before calling this function.
func (w *World) GetChunk(pos protocol.ChunkPos) ChunkSource {
	w.RLock()
	c := w.chunks[pos]
	w.RUnlock()

	return c
}

// Block returns the block at the position passed.
func (w *World) Block(pos df_cube.Pos) world.Block {
	blockPos := cube.Pos(pos)
	if blockPos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return block.Air{}
	}

	chunkPos := protocol.ChunkPos{int32(blockPos[0]) >> 4, int32(blockPos[2]) >> 4}
	w.RLock()
	blockUpdates, found := w.blockUpdates[chunkPos]
	w.RUnlock()
	if found {
		if b, ok := blockUpdates[df_cube.Pos(blockPos)]; ok {
			return b
		}
	} else {
		w.Lock()
		w.blockUpdates[chunkPos] = make(map[df_cube.Pos]world.Block)
		w.Unlock()
	}

	c := w.GetChunk(chunkPos)
	if c == nil {
		return block.Air{}
	}

	// TODO: Implement and account for multi-layer blocks.
	rid := c.Block(uint8(blockPos[0]), int16(blockPos[1]), uint8(blockPos[2]), 0)
	if b, ok := world.BlockByRuntimeID(rid); ok {
		return b
	}
	return block.Air{}
}

// SetBlock sets the block at the position passed.
func (w *World) SetBlock(pos df_cube.Pos, b world.Block, _ *world.SetOpts) {
	if cube.Pos(pos).OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}
	chunkPos := protocol.ChunkPos{int32(pos[0]) >> 4, int32(pos[2]) >> 4}

	w.Lock()
	defer w.Unlock()

	if w.blockUpdates[chunkPos] == nil {
		w.blockUpdates[chunkPos] = make(map[df_cube.Pos]world.Block)
	}
	w.blockUpdates[chunkPos][pos] = b
}

// CleanChunks cleans up the chunks in respect to the given chunk radius and chunk position.
func (w *World) CleanChunks(radius int32, pos protocol.ChunkPos) {
	w.Lock()
	defer w.Unlock()

	if pos == w.lastCleanPos {
		return
	}
	w.lastCleanPos = pos

	for chunkPos, c := range w.chunks {
		_, exempted := w.exemptedChunks[chunkPos]
		inRange := chunkInRange(radius, chunkPos, pos)

		if exempted && inRange {
			if w.logger != nil {
				(*w.logger).Info("removed exempted chunk stats", "chunkPos", chunkPos, "radius", radius, "pos", pos)
			}
			delete(w.exemptedChunks, chunkPos)
		} else if !exempted && !inRange {
			if cached, ok := c.(*CachedChunk); ok {
				cached.Unsubscribe()
			}
			delete(w.chunks, chunkPos)
			delete(w.blockUpdates, chunkPos)
			if w.logger != nil {
				(*w.logger).Info("removed non-exempted chunk stats", "chunkPos", chunkPos, "radius", radius, "pos", pos)
			}
		}
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
	w.Lock()
	defer w.Unlock()

	for chunkPos, c := range w.chunks {
		if cached, ok := c.(*CachedChunk); ok {
			cached.Unsubscribe()
		}
		delete(w.chunks, chunkPos)
	}
}

// chunkInRange returns true if the chunk position is within the given radius of the chunk position.
func chunkInRange(radius int32, chunkPos, pos protocol.ChunkPos) bool {
	diffX, diffZ := pos[0]-chunkPos[0], pos[1]-chunkPos[1]
	dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

	return int32(dist) <= radius
}
