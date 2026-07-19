package utils

import (
	"bytes"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/world/chunk"
)

// noinspection ALL
//
//go:linkname DecodeSubChunk github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func DecodeSubChunk(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
