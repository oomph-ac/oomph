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

// nopLogger is a no-op implementation of the Logger interface to suppress logging on worlds/expected missing chunks.
type nopLogger struct{}

func (n nopLogger) Debugf(string, ...interface{}) {}
func (n nopLogger) Infof(string, ...interface{})  {}
func (n nopLogger) Errorf(string, ...interface{}) {}
func (n nopLogger) Fatalf(string, ...interface{}) {}

// loadRawChunk loads a chunk to the player's world provider from raw sub chunk data.
func (p *Player) loadRawChunk(pos world.ChunkPos, data []byte, subChunkCount uint32) {
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

// tickNearbyBlocks is called once every client tick to update block ticks.
//func (p *Player) tickNearbyBlocks() {
//	aabb := p.Entity().AABB().Grow(0.2).Translate(p.Position())
//
//	var liquids, climbables uint32
//	for _, v := range utils.DefaultCheckBlockSettings(aabb, p).SearchAll() {
//		// TODO: Also check for vines and cobwebs when added in DF.
//		switch v.(type) {
//		case world.Liquid:
//			liquids++
//		case block.Ladder:
//			climbables++
//		}
//	}
//
//	p.spawnTicks++
//	p.liquidTicks++
//	p.motionTicks++
//	p.climbableTicks++
//	if p.dead {
//		p.spawnTicks = 0
//	}
//	if liquids > 0 {
//		p.liquidTicks = 0
//	}
//	if climbables > 0 {
//		p.climbableTicks = 0
//	}
//}
