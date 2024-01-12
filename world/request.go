package world

import (
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type AddChunkRequest struct {
	c   *chunk.Chunk
	pos protocol.ChunkPos
	w   *World
}
