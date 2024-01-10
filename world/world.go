package world

import (
	"sync"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
	chunks map[protocol.ChunkPos]uint64
	id     uint64

	sync.Mutex
}

func NewWorld() *World {
	currentWorldId++
	return &World{
		chunks: make(map[protocol.ChunkPos]uint64),
		id:     currentWorldId,
	}
}

// AddChunk adds a chunk to the world.
func (w *World) AddChunk(pos protocol.ChunkPos, c *chunk.Chunk) {
	w.Lock()
	defer w.Unlock()

	w.chunks[pos] = InsertIntoCache(w, pos, c)
}

// GetChunk returns a cached chunk at the position passed. The mutex is
// not locked here because it is assumed that the caller has already locked
// the mutex before calling this function.
func (w *World) GetChunk(pos protocol.ChunkPos) *CachedChunk {
	chunkID, ok := w.chunks[pos]
	if !ok {
		return nil
	}

	return SearchFromCache(pos, chunkID)
}

// GetBlock returns the block at the position passed.
func (w *World) GetBlock(pos mgl32.Vec3) world.Block {
	w.Lock()
	defer w.Unlock()

	blockPos := cube.Pos{int(pos[0]), int(pos[1]), int(pos[2])}
	if blockPos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return block.Air{}
	}

	chunkPos := protocol.ChunkPos{int32(blockPos[0]) / 16, int32(blockPos[2]) / 16}
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
func (w *World) SetBlock(pos cube.Pos, b world.Block) {
	w.Lock()
	defer w.Unlock()

	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}

	blockID := world.BlockRuntimeID(b)
	chunkPos := protocol.ChunkPos{int32(pos[0]) / 16, int32(pos[2]) / 16}
	c := w.GetChunk(chunkPos)
	if c == nil {
		return
	}

	c.Execute(w, chunkPos, SetBlockTransaction{
		BlockPos:       pos,
		BlockRuntimeId: blockID,
	})
}

// CleanChunks cleans up the chunks in respect to the given chunk radius and chunk position.
func (w *World) CleanChunks(radius int32, pos protocol.ChunkPos) {
	w.Lock()
	defer w.Unlock()

	for chunkPos, id := range w.chunks {
		if chunkInRange(radius, chunkPos, pos) {
			continue
		}

		cached := SearchFromCache(chunkPos, id)
		if cached != nil {
			cached.Unsubscribe(w)
		}

		delete(w.chunks, chunkPos)
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
	for pos, id := range w.chunks {
		cached := SearchFromCache(pos, id)
		if cached != nil {
			cached.Unsubscribe(w)
		}
	}

	maps.Clear(w.chunks)
}

// chunkInRange returns true if the chunk position is within the given radius of the chunk position.
func chunkInRange(radius int32, cpos, pos protocol.ChunkPos) bool {
	diffX, diffZ := pos[0]-cpos[0], pos[1]-cpos[1]
	dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

	return int32(dist) <= radius
}
