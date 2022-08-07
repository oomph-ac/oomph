package player

import (
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// LoadChunk adds a chunk to the map of chunks
func (p *Player) LoadChunk(pos protocol.ChunkPos, c *chunk.Chunk) {
	p.chkMu.Lock()
	p.chunks[pos] = c
	p.chkMu.Unlock()
}

// UnloadChunk removes a chunk from the map of chunks
func (p *Player) UnloadChunk(pos protocol.ChunkPos) {
	p.chkMu.Lock()
	delete(p.chunks, pos)
	p.chkMu.Unlock()
}

// ChunkExists returns true if the given chunk position was found in the map of chunks
func (p *Player) ChunkExists(pos protocol.ChunkPos) bool {
	p.chkMu.Lock()
	_, ok := p.chunks[pos]
	p.chkMu.Unlock()
	return ok
}

// Chunk returns a chunk from the given chunk position. If the chunk was found in the map, it will
// return the chunk and true
func (p *Player) Chunk(pos protocol.ChunkPos) (*chunk.Chunk, bool) {
	p.chkMu.Lock()
	c, ok := p.chunks[pos]
	p.chkMu.Unlock()
	if ok {
		c.Lock()
		return c, ok
	}

	return nil, ok
}

// Block returns the block found at the given position
func (p *Player) Block(pos cube.Pos) world.Block {
	if pos.OutOfBounds(world.Overworld.Range()) {
		return block.Air{}
	}
	c, ok := p.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		return block.Air{}
	}
	rid := c.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)
	c.Unlock()

	b, _ := world.BlockByRuntimeID(rid)
	return b
}

// SetBlock sets a block at the given position to the given block
func (p *Player) SetBlock(pos cube.Pos, b world.Block) {
	if pos.OutOfBounds(world.Overworld.Range()) {
		return
	}

	rid := world.BlockRuntimeID(b)
	c, ok := p.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		return
	}

	c.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, rid)
	c.Unlock()
}

// GetNearbyBBoxes returns a list of block ounding boxes that are within the given bounding box - which is usually
// the player's bounding box.
func (p *Player) GetNearbyBBoxes(aabb cube.BBox) []cube.BBox {
	grown := aabb.Grow(1)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math.Floor(min[0])), int(math.Floor(min[1])), int(math.Floor(min[2]))
	maxX, maxY, maxZ := int(math.Ceil(max[0])), int(math.Ceil(max[1])), int(math.Ceil(max[2]))

	// A prediction of one BBox per block, plus an additional 2, in case
	var blockBBoxs []cube.BBox
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				boxes := p.Block(pos).Model().BBox(pos, nil)
				for _, box := range boxes {
					if box.Translate(pos.Vec3()).IntersectsWith(aabb) {
						blockBBoxs = append(blockBBoxs, box.Translate(pos.Vec3()))
					}
				}
			}
		}
	}
	return blockBBoxs
}

// GetNearbyBlocks returns a list of blocks that are within the given bounding box.
func (p *Player) GetNearbyBlocks(aabb cube.BBox) []world.Block {
	grown := aabb.Grow(0.25)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math.Floor(min[0])), int(math.Floor(min[1])), int(math.Floor(min[2]))
	maxX, maxY, maxZ := int(math.Ceil(max[0])), int(math.Ceil(max[1])), int(math.Ceil(max[2]))

	// A prediction of one BBox per block, plus an additional 2, in case
	var blocks []world.Block
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				blocks = append(blocks, p.Block(pos))
			}
		}
	}
	return blocks
}

// cleanChunks filters out any chunks that are out of the player's view, and returns a value of
// how many chunks were cleaned
func (p *Player) cleanChunks() (cleaned int) {
	p.chkMu.Lock()
	defer p.chkMu.Unlock()

	loc := p.mInfo.ServerPosition
	activePos := world.ChunkPos{int32(math.Floor(loc[0])) >> 4, int32(math.Floor(loc[2])) >> 4}
	for pos := range p.chunks {
		diffX, diffZ := pos[0]-activePos[0], pos[1]-activePos[1]
		dist := math.Sqrt(float64(diffX*diffX) + float64(diffZ*diffZ))
		if int(dist) > p.chunkRadius {
			delete(p.chunks, pos)
			cleaned++
		}
	}

	return
}
