package world

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/zeebo/xxh3"

	_ "unsafe"

	_ "github.com/oomph-ac/oomph/world/block"
)

type ChunkInfo struct {
	Cached bool
	Hash   xxh3.Uint128
	Chunk  *chunk.Chunk
}

type World struct {
	lastCleanPos protocol.ChunkPos

	chunks    map[protocol.ChunkPos]ChunkInfo
	subChunks map[protocol.ChunkPos][]xxh3.Uint128

	exemptedChunks map[protocol.ChunkPos]struct{}
	blockUpdates   map[protocol.ChunkPos]map[df_cube.Pos]world.Block

	debugFn func(string, ...any)

	stwTicks uint16
}

func New(debugFn func(string, ...any)) *World {
	return &World{
		chunks:    make(map[protocol.ChunkPos]ChunkInfo),
		subChunks: make(map[protocol.ChunkPos][]xxh3.Uint128),

		exemptedChunks: make(map[protocol.ChunkPos]struct{}),
		blockUpdates:   make(map[protocol.ChunkPos]map[df_cube.Pos]world.Block),

		debugFn: debugFn,
	}
}

func (w *World) SetSTWTicks(ticks uint16) {
	w.stwTicks = ticks
}

// AddChunk adds a chunk to the world.
func (w *World) AddChunk(chunkPos protocol.ChunkPos, c ChunkInfo) {
	if oldChunkInfo, ok := w.chunks[chunkPos]; ok {
		w.removeChunk(oldChunkInfo, chunkPos)
	}
	w.chunks[chunkPos] = c
	w.exemptedChunks[chunkPos] = struct{}{}
}

// AddSubChunk adds a subchunk to the world.
func (w *World) AddSubChunk(chunkPos protocol.ChunkPos, hash xxh3.Uint128) {
	if _, ok := w.subChunks[chunkPos]; !ok {
		w.subChunks[chunkPos] = make([]xxh3.Uint128, 0, 16)
	}
	w.subChunks[chunkPos] = append(w.subChunks[chunkPos], hash)
}

// GetChunk returns a cached chunk at the position passed. The mutex is
// not locked here because it is assumed that the caller has already locked
// the mutex before calling this function.
func (w *World) GetChunk(pos protocol.ChunkPos) *chunk.Chunk {
	if info, ok := w.chunks[pos]; ok {
		return info.Chunk
	}
	return nil
}

// Block returns the block at the position passed.
func (w *World) Block(pos df_cube.Pos) world.Block {
	blockPos := cube.Pos(pos)
	if blockPos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return block.Air{}
	}

	chunkPos := protocol.ChunkPos{int32(blockPos[0]) >> 4, int32(blockPos[2]) >> 4}
	blockUpdates, found := w.blockUpdates[chunkPos]
	if found {
		if b, ok := blockUpdates[df_cube.Pos(blockPos)]; ok {
			return b
		}
	} else {
		w.blockUpdates[chunkPos] = make(map[df_cube.Pos]world.Block)
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
	if w.blockUpdates[chunkPos] == nil {
		w.blockUpdates[chunkPos] = make(map[df_cube.Pos]world.Block)
	}
	w.blockUpdates[chunkPos][pos] = b
}

// CleanChunks cleans up the chunks in respect to the given chunk radius and chunk position.
func (w *World) CleanChunks(radius int32, pos protocol.ChunkPos) {
	debugFn := w.debugFn
	if w.stwTicks > 0 {
		if debugFn != nil {
			debugFn("not cleaning chunks - stwTicks=%d", w.stwTicks)
		}
		w.stwTicks--
		return
	}

	if pos == w.lastCleanPos {
		return
	}
	w.lastCleanPos = pos

	for chunkPos, c := range w.chunks {
		_, exempted := w.exemptedChunks[chunkPos]
		inRange := chunkInRange(radius, chunkPos, pos)

		if exempted && inRange {
			if debugFn != nil {
				debugFn("removed exempted chunk stats chunkPos=%v, radius=%d, pos=%v", chunkPos, radius, pos)
			}
			delete(w.exemptedChunks, chunkPos)
		} else if !exempted && !inRange {
			if debugFn != nil {
				debugFn("removed non-exempted chunk stats chunkPos=%v, radius=%d, pos=%v", chunkPos, radius, pos)
			}
			w.removeChunk(c, chunkPos)
		}
	}
}

// PurgeChunks removes all chunks from the world.
func (w *World) PurgeChunks() {
	for chunkPos, cInfo := range w.chunks {
		w.removeChunk(cInfo, chunkPos)
	}
}

func (w *World) removeChunk(info ChunkInfo, chunkPos protocol.ChunkPos) {
	if info.Cached {
		unsubFromChunk(info.Hash)
	}
	if subChunks, ok := w.subChunks[chunkPos]; ok {
		for _, subChunkHash := range subChunks {
			unsubFromSubChunk(subChunkHash)
		}
	}
	delete(w.subChunks, chunkPos)
	delete(w.chunks, chunkPos)
	delete(w.blockUpdates, chunkPos)
}

// chunkInRange returns true if the chunk position is within the given radius of the chunk position.
func chunkInRange(radius int32, chunkPos, pos protocol.ChunkPos) bool {
	diffX, diffZ := pos[0]-chunkPos[0], pos[1]-chunkPos[1]
	dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

	return int32(dist) <= radius
}
