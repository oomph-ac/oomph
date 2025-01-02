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

	deferredBlocks map[df_cube.Pos]world.Block
	dbMu           sync.Mutex

	lastCleanPos protocol.ChunkPos

	deadlock.RWMutex
}

func New() *World {
	currentWorldId++
	return &World{
		chunks:         make(map[protocol.ChunkPos]ChunkSource),
		exemptedChunks: make(map[protocol.ChunkPos]struct{}),
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
	}
	w.chunks[chunkPos] = c
	w.Unlock()

	w.dbMu.Lock()
	for blockPos, b := range w.deferredBlocks {
		blockChunk := protocol.ChunkPos{int32(blockPos[0] >> 4), int32(blockPos[2] >> 4)}
		if blockChunk == chunkPos {
			w.SetBlock(blockPos, b, nil)
			delete(w.deferredBlocks, blockPos)
		}
	}
	w.dbMu.Unlock()
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

	blockID := world.BlockRuntimeID(b)
	chunkPos := protocol.ChunkPos{int32(pos[0]) >> 4, int32(pos[2]) >> 4}
	c := w.GetChunk(chunkPos)
	if c == nil {
		// If the given chunk is not found, but is in the exempted list, this is casued by a small delay
		// in the cache workers adding the chunk to the world.
		w.Lock()
		if _, exempted := w.exemptedChunks[chunkPos]; exempted {
			w.dbMu.Lock()
			w.deferredBlocks[pos] = b
			w.dbMu.Unlock()
		}
		w.Unlock()
		return
	}

	// Check if the current chunk source is a SubscribedChunk (it's cached), and if so, make a copy
	// of the chunk and replace it in the map with a local chunk.
	if cachedChunk, ok := c.(*CachedChunk); ok {
		cachedChunk.Unsubscribe()
		originalChunk := *(cachedChunk.c)
		c = &originalChunk

		w.Lock()
		w.chunks[chunkPos] = c
		w.Unlock()
	}

	if originalChunk, ok := c.(*chunk.Chunk); ok {
		// TODO: Implement chunk layers properly.
		originalChunk.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, blockID)
	} else {
		panic(oerror.New("unexpected ChunkSource in world: %T", c))
	}
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
