package player

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"math"
)

// LoadRawChunk loads a chunk to the player's memory from raw sub chunk data.
func (p *Player) LoadRawChunk(pos world.ChunkPos, data []byte, subChunkCount uint32) {
	if p.closed {
		// Don't load a chunk if the player is already closed.
		return
	}

	a, _ := chunk.StateToRuntimeID("minecraft:air", nil)
	ch, err := chunk.NetworkDecode(a, data, int(subChunkCount), p.dimension.Range())
	if err != nil {
		p.log.Errorf("failed to parse chunk at %v: %v", pos, err)
		return
	}
	ch.Compact()

	p.LoadChunk(pos, ch)
}

// LoadChunk loads a chunk to the player's memory.
func (p *Player) LoadChunk(pos world.ChunkPos, c *chunk.Chunk) {
	if p.closed {
		// Don't load a chunk if the player is already closed.
		return
	}

	p.chunkMu.Lock()
	p.chunks[pos] = c
	p.chunkMu.Unlock()
}

// UnloadChunk unloads a chunk in the player's memory.
func (p *Player) UnloadChunk(pos world.ChunkPos) {
	p.chunkMu.Lock()
	delete(p.chunks, pos)
	p.chunkMu.Unlock()
}

// Chunk attempts to return a chunk in the player's memory. If it does not exist, the second return value will be false.
func (p *Player) Chunk(pos world.ChunkPos) (*chunk.Chunk, bool) {
	p.chunkMu.Lock()
	defer p.chunkMu.Unlock()
	if c, ok := p.chunks[pos]; ok {
		c.Lock()
		return c, ok
	}
	return nil, false
}

// Block reads a block from the position passed. If a chunk is not yet loaded at that position, it will return air.
func (p *Player) Block(pos cube.Pos) world.Block {
	if pos.OutOfBounds(p.dimension.Range()) {
		return block.Air{}
	}
	c, ok := p.Chunk(world.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		p.log.Errorf("failed to query chunk at %v", pos)
		return block.Air{}
	}
	rid := c.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)
	c.Unlock()

	b, _ := world.BlockByRuntimeID(rid)
	return b
}

// SetBlock writes a block to the position passed. If a chunk is not yet loaded at that position, it will do nothing.
func (p *Player) SetBlock(pos cube.Pos, b world.Block) {
	if pos.OutOfBounds(p.dimension.Range()) {
		return
	}

	rid, ok := world.BlockRuntimeID(b)
	if !ok {
		p.log.Errorf("failed to query runtime id for %v at %v", rid, pos)
		return
	}

	c, ok := p.Chunk(world.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		p.log.Errorf("failed to query chunk at %v", pos)
		return
	}
	c.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, rid)
	c.Unlock()
}

// tickNearbyBlocks is called once every client tick to update block ticks.
func (p *Player) tickNearbyBlocks() {
	aabb := p.Entity().AABB().Grow(0.2).Translate(p.Entity().Position())

	var liquids, climbables uint32
	for _, v := range utils.DefaultCheckBlockSettings(aabb, p).SearchAll() {
		// TODO: Also check for vines and cobwebs when added in DF.
		switch v.(type) {
		case world.Liquid:
			liquids++
		case block.Ladder:
			climbables++
		}
	}

	p.spawnTicks++
	p.liquidTicks++
	p.motionTicks++
	p.climbableTicks++
	if p.dead {
		p.spawnTicks = 0
	}
	if liquids > 0 {
		p.liquidTicks = 0
	}
	if climbables > 0 {
		p.climbableTicks = 0
	}
}

// cleanChunks removes all cached chunks that are no longer in the player's view.
func (p *Player) cleanChunks() {
	if p.closed {
		// Don't clean chunks if the player is already closed.
		return
	}

	p.chunkMu.Lock()
	defer p.chunkMu.Unlock()

	loc := p.Entity().Position()
	activePos := world.ChunkPos{int32(math.Floor(loc[0])) >> 4, int32(math.Floor(loc[2])) >> 4}
	for pos := range p.chunks {
		diffX, diffZ := pos[0]-activePos[0], pos[1]-activePos[1]
		dist := math.Sqrt(float64(diffX*diffX) + float64(diffZ*diffZ))
		if int32(dist) > p.viewDist {
			delete(p.chunks, pos)
		}
	}
}
