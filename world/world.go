package world

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
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
	chunks map[protocol.ChunkPos]*CachedChunk
	id     uint64

	ghostBlocks     map[cube.Pos]world.Block
	searchWithGhost bool

	lastCleanPos protocol.ChunkPos

	deadlock.RWMutex
}

func New() *World {
	currentWorldId++
	return &World{
		chunks:      make(map[protocol.ChunkPos]*CachedChunk),
		ghostBlocks: make(map[cube.Pos]world.Block),
		id:          currentWorldId,
	}
}

// SearchWithGhost sets wether the world should search for ghost blocks or not while calling GetBlock.
func (w *World) SearchWithGhost(search bool) {
	w.searchWithGhost = search
}

// HasGhostBlocks returns true if the world has ghost blocks.
func (w *World) HasGhostBlocks() bool {
	return len(w.ghostBlocks) > 0
}

// AddChunk adds a chunk to the world.
func (w *World) AddChunk(c *CachedChunk) {
	w.Lock()
	defer w.Unlock()

	w.chunks[c.Pos] = c
}

// RemoveChunk removes a chunk from the world.
func (w *World) RemoveChunk(pos protocol.ChunkPos) {
	w.Lock()
	defer w.Unlock()

	delete(w.chunks, pos)
}

// GetChunk returns a cached chunk at the position passed. The mutex is
// not locked here because it is assumed that the caller has already locked
// the mutex before calling this function.
func (w *World) GetChunk(pos protocol.ChunkPos) *CachedChunk {
	w.RLock()
	defer w.RUnlock()

	return w.chunks[pos]
}

// GetBlock returns the block at the position passed.
func (w *World) GetBlock(blockPos cube.Pos) world.Block {
	if b, ok := w.ghostBlocks[blockPos]; ok && w.searchWithGhost {
		return b
	}

	if blockPos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return block.Air{}
	}

	chunkPos := protocol.ChunkPos{int32(blockPos[0]) >> 4, int32(blockPos[2]) >> 4}
	c := w.GetChunk(chunkPos)
	if c == nil {
		return block.Air{}
	}

	// Validate that the block position is within the chunk.
	if c.Pos.X() != (int32(blockPos[0])>>4) || c.Pos.Z() != (int32(blockPos[2])>>4) {
		panic(oerror.New("world.GetBlock: GetChunk() returned an invalid chunk"))
	}

	c.RLock()
	rid := c.Block(uint8(blockPos[0]), int16(blockPos[1]), uint8(blockPos[2]), 0)
	c.RUnlock()

	b, ok := world.BlockByRuntimeID(rid)
	if !ok {
		return block.Air{}
	}
	return b
}

// SetBlock sets the block at the position passed.
func (w *World) SetBlock(pos cube.Pos, b world.Block) {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}

	blockID := world.BlockRuntimeID(b)
	chunkPos := protocol.ChunkPos{int32(pos[0]) >> 4, int32(pos[2]) >> 4}
	c := w.GetChunk(chunkPos)
	if c == nil {
		return
	}

	c.ActionSetBlock(w, SetBlockAction{
		BlockPos:       pos,
		BlockRuntimeId: blockID,
	})
}

// MarkGhostBlock marks a block as a ghost block at the position passed.
func (w *World) MarkGhostBlock(pos cube.Pos, b world.Block) {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}

	w.ghostBlocks[pos] = b
}

// RemoveGhostBlock removes a ghost block at the position passed.
func (w *World) RemoveGhostBlock(pos cube.Pos) {
	delete(w.ghostBlocks, pos)
}

// UnmarkGhostBLock unmarks a ghost block at the position passed.
func (w *World) UnmarkGhostBlock(pos cube.Pos) {
	delete(w.ghostBlocks, pos)
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
		if chunkInRange(radius, chunkPos, pos) {
			continue
		}

		// We have to temporarily unlock the world mutex here to avoid a deadlock when *CachedChunk.Unsubscribe is called
		// This is so ugly holy shit kill me.
		w.Unlock()
		c.Unsubscribe(w)
		w.Lock()
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
	for _, c := range w.chunks {
		c.Unsubscribe(w)
	}

	maps.Clear(w.chunks)
}

// chunkInRange returns true if the chunk position is within the given radius of the chunk position.
func chunkInRange(radius int32, cpos, pos protocol.ChunkPos) bool {
	diffX, diffZ := pos[0]-cpos[0], pos[1]-cpos[1]
	dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

	return int32(dist) <= radius
}
