package world

import (
	"bytes"

	_ "unsafe"

	"github.com/df-mc/dragonfly/server/world/chunk"
)

// ChunkSource is an interface that returns block information like a regular chunk.
type ChunkSource interface {
	// Block returns the block at the given position and layer of the chunk source.
	Block(x uint8, y int16, z uint8, layer uint8) (rid uint32)
}

// noinspection ALL
//
//go:linkname decodeSubChunk github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func decodeSubChunk(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
