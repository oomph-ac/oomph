package world

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sasha-s/go-deadlock"
	"golang.org/x/exp/maps"
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
	chunks         map[protocol.ChunkPos]*chunk.Chunk
	exemptedChunks map[protocol.ChunkPos]struct{}

	id uint64

	lastCleanPos protocol.ChunkPos

	deadlock.RWMutex
}

func New() *World {
	currentWorldId++
	return &World{
		chunks:         make(map[protocol.ChunkPos]*chunk.Chunk),
		exemptedChunks: make(map[protocol.ChunkPos]struct{}),

		id: currentWorldId,
	}
}

// AddChunk adds a chunk to the world.
func (w *World) AddChunk(pos protocol.ChunkPos, c *chunk.Chunk) {
	w.Lock()
	w.chunks[pos] = c
	w.Unlock()
}

// RemoveChunk removes a chunk from the world.
func (w *World) RemoveChunk(pos protocol.ChunkPos) {
	w.Lock()
	defer w.Unlock()

	delete(w.chunks, pos)
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
func (w *World) GetChunk(pos protocol.ChunkPos) *chunk.Chunk {
	w.RLock()
	c := w.chunks[pos]
	w.RUnlock()

	return c
}

// GetAllChunks returns all chunks in the world.
func (w *World) GetAllChunks() map[protocol.ChunkPos]*chunk.Chunk {
	w.RLock()
	defer w.RUnlock()

	return w.chunks
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
		return
	}

	// TODO: Implement layers properly.
	c.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, blockID)
}

// CleanChunks cleans up the chunks in respect to the given chunk radius and chunk position.
func (w *World) CleanChunks(radius int32, pos protocol.ChunkPos) {
	w.Lock()
	defer w.Unlock()

	if pos == w.lastCleanPos {
		return
	}
	w.lastCleanPos = pos

	for chunkPos := range w.chunks {
		_, exempted := w.exemptedChunks[chunkPos]
		inRange := chunkInRange(radius, chunkPos, pos)

		if exempted && inRange {
			delete(w.exemptedChunks, chunkPos)
		} else if !exempted && !inRange {
			delete(w.chunks, chunkPos)
		}
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
	maps.Clear(w.chunks)
}

// chunkInRange returns true if the chunk position is within the given radius of the chunk position.
func chunkInRange(radius int32, cpos, pos protocol.ChunkPos) bool {
	diffX, diffZ := pos[0]-cpos[0], pos[1]-cpos[1]
	dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

	return int32(dist) <= radius
}
