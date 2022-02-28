package player

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"sync"
)

// provider represents a chunk provider for a player's world. It stores chunks temporarily until the w
type provider struct {
	chunkMu sync.Mutex
	chunks  map[world.ChunkPos]*chunk.Chunk

	world.NoIOProvider
}

// LoadChunk ...
func (p *provider) LoadChunk(pos world.ChunkPos) (*chunk.Chunk, bool, error) {
	p.chunkMu.Lock()
	defer p.chunkMu.Unlock()

	ch, ok := p.chunks[pos]
	if !ok {
		return nil, false, fmt.Errorf("could not load chunk: was not found")
	}
	delete(p.chunks, pos)
	return ch, ok, nil
}

// loadChunk loads a chunk to the player's world provider from raw sub chunk data.
func (p *Player) loadChunk(pos world.ChunkPos, data []byte, subChunkCount uint32) {
	if p.closed {
		// Don't load a chunk if the player is already closed.
		return
	}

	a, _ := chunk.StateToRuntimeID("minecraft:air", nil)
	ch, err := chunk.NetworkDecode(a, data, int(subChunkCount), p.w.Range())
	if err != nil {
		p.log.Errorf("failed to parse chunk at %v: %v", pos, err)
		return
	}
	ch.Compact()

	p.wMu.Lock()
	p.p.chunkMu.Lock()
	p.p.chunks[pos] = ch
	p.p.chunkMu.Unlock()
	p.wMu.Unlock()
}
