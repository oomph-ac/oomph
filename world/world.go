package world

import (
	"sync"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sasha-s/go-deadlock"
)

var AirRuntimeID uint32

func init() {
	a, ok := chunk.StateToRuntimeID("minecraft:air", nil)
	if !ok {
		panic(oerror.New("unable to find runtime ID for air"))
	}
	AirRuntimeID = a
}

var currentWorldId uint64

type World struct {
	id uint64

	chunks         map[protocol.ChunkPos]ChunkSource
	exemptedChunks map[protocol.ChunkPos]struct{}
	blockUpdates   map[protocol.ChunkPos]map[df_cube.Pos]world.Block

	deferredBlocks map[df_cube.Pos]world.Block
	blocksMu       sync.Mutex

	lastCleanPos protocol.ChunkPos

	deadlock.RWMutex
}

func New() *World {
	currentWorldId++
	return &World{
		chunks:         make(map[protocol.ChunkPos]ChunkSource),
		exemptedChunks: make(map[protocol.ChunkPos]struct{}),
		blockUpdates:   make(map[protocol.ChunkPos]map[df_cube.Pos]world.Block),

		deferredBlocks: make(map[df_cube.Pos]world.Block),

		id: currentWorldId,
	}
}

// AddChunk adds a chunk to the world.
func (w *World) AddChunk(chunkPos protocol.ChunkPos, c ChunkSource) {
	w.Lock()
	if old, ok := w.chunks[chunkPos]; ok {
		if cached, ok := old.(*CachedChunk); ok {
			cached.Unsubscribe()
		}
		delete(w.blockUpdates, chunkPos)
	}
	w.chunks[chunkPos] = c
	w.Unlock()

	w.blocksMu.Lock()
	for blockPos, b := range w.deferredBlocks {
		blockChunk := protocol.ChunkPos{int32(blockPos[0] >> 4), int32(blockPos[2] >> 4)}
		if blockChunk == chunkPos {
			w.SetBlock(blockPos, b, nil)
			delete(w.deferredBlocks, blockPos)
		}
	}
	w.blocksMu.Unlock()
}

// ExemptChunk adds a chunk to the exemption list. This exemption is removed when
// the player is within range of the chunk, which is handled in World.CleanChunks()
func (w *World) ExemptChunk(pos protocol.ChunkPos) {
	w.Lock()
	w.exemptedChunks[pos] = struct{}{}
	w.Unlock()
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
	rid := c.Block(uint8(blockPos[0]), int16(blockPos[1]), uint8(blockPos[2]), 0)

	b, ok := world.BlockByRuntimeID(rid)
	if !ok {
		return block.Air{}
	}
	return b
}

// SetBlock sets the block at the position passed.
func (w *World) SetBlock(pos df_cube.Pos, b world.Block, _ *world.SetOpts) {
	if cube.Pos(pos).OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}

	chunkPos := protocol.ChunkPos{int32(pos[0]) >> 4, int32(pos[2]) >> 4}
	c := w.GetChunk(chunkPos)
	if c == nil {
		// If the given chunk is not found, but is in the exempted list, this is casued by a small delay
		// in the cache workers adding the chunk to the world.
		w.Lock()
		if _, exempted := w.exemptedChunks[chunkPos]; exempted {
			w.blocksMu.Lock()
			w.deferredBlocks[pos] = b
			w.blocksMu.Unlock()
		}
		w.Unlock()
		return
	}

	w.Lock()
	if w.blockUpdates[chunkPos] == nil {
		w.blockUpdates[chunkPos] = make(map[df_cube.Pos]world.Block)
	}
	w.blockUpdates[chunkPos][pos] = b
	w.Unlock()
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
			delete(w.exemptedChunks, chunkPos)
		} else if !exempted && !inRange {
			if cached, ok := c.(*CachedChunk); ok {
				cached.Unsubscribe()
			}
			delete(w.chunks, chunkPos)
			delete(w.blockUpdates, chunkPos)
		}
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
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
