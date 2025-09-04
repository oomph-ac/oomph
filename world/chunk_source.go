package world

import (
	"bytes"

	_ "unsafe"

	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

// ChunkSource is an interface that returns block information like a regular chunk.
type ChunkSource interface {
	// Block returns the block at the given position and layer of the chunk source.
	Block(x uint8, y int16, z uint8, layer uint8) (rid uint32)
	// BlockNBT returns the NBT data of the block at the given position.
	BlockNBT(pos df_cube.Pos) (world.Block, bool)
}

// noinspection ALL
//
//go:linkname decodeSubChunk github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func decodeSubChunk(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
