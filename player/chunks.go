package player

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// GetChunkPos returns the chunk position from the given x and z coordinates.
func GetChunkPos(x, z int32) protocol.ChunkPos {
	return protocol.ChunkPos{x >> 4, z >> 4}
}

// ChunkExists returns true if the given chunk position was found in the map of chunks
func (p *Player) ChunkExists(pos protocol.ChunkPos) bool {
	_, ok := p.Chunk(pos)
	return ok
}

// AddChunk adds a chunk to the chunk map of the player. This function can also be used to replace existing chunks
func (p *Player) AddChunk(c *chunk.Chunk, pos protocol.ChunkPos) {
	p.chunkMu.Lock()
	defer p.chunkMu.Unlock()

	p.chunks[pos] = c
}

// Chunk returns a chunk from the given chunk position. If the chunk was found in the map, it will
// return the chunk and true.
func (p *Player) Chunk(pos protocol.ChunkPos) (*chunk.Chunk, bool) {
	p.chunkMu.Lock()
	defer p.chunkMu.Unlock()

	c, ok := p.chunks[pos]
	if !ok {
		return nil, false
	}

	return c, true
}

// Block returns the block found at the given position
func (p *Player) Block(pos cube.Pos) world.Block {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return block.Air{}
	}

	// Check the block result cache to see if we already have this block.
	if v, ok := p.cachedBlockResults.Load(pos); ok {
		return v.(world.Block)
	}

	c, ok := p.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})

	if !ok {
		return block.Air{}
	}
	rid := c.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)

	b, _ := world.BlockByRuntimeID(rid)
	p.cachedBlockResults.Store(pos, b)

	return b
}

// SurroundingBlocks gets blocks that surround the given position, not including air.
func (p *Player) SurroundingBlocks(pos cube.Pos) map[cube.Face]world.Block {
	surrounds := make(map[cube.Face]world.Block)

	for _, face := range cube.Faces() {
		b := p.Block(pos.Side(face))
		n, _ := b.EncodeBlock()
		if n == "minecraft:air" {
			continue
		}

		surrounds[face] = b
	}

	return surrounds
}

// GetTargetBlock checks if the user's ray interesects with any blocks, which may prevent
// them from interacting with entities for combat.
func (p *Player) GetTargetBlock(ray mgl32.Vec3, pos mgl32.Vec3, dist float32) (world.Block, float32) {
	blockMap := make(map[cube.Pos]world.Block)
	for i := float32(0); i <= dist; i += 0.2 {
		bpos := cube.PosFromVec3(pos.Add(ray.Mul(i)))
		blockMap[bpos] = p.Block(bpos)
	}

	for bpos, b := range blockMap {
		bbs := utils.BlockBoxes(b, bpos, p.SurroundingBlocks(bpos))
		for _, bb := range bbs {
			res, ok := trace.BBoxIntercept(bb.Translate(mgl32.Vec3{float32(bpos.X()), float32(bpos.Y()), float32(bpos.Z())}), pos, pos.Add(ray.Mul(dist)))
			if !ok {
				continue
			}

			return b, res.Position().Sub(pos).Len()
		}
	}

	return nil, 0
}

// SetBlock sets a block at the given position to the given block
func (p *Player) SetBlock(pos cube.Pos, b world.Block) {
	if pos.OutOfBounds(cube.Range(world.Overworld.Range())) {
		return
	}

	rid := world.BlockRuntimeID(b)
	c, ok := p.Chunk(protocol.ChunkPos{int32(pos[0] >> 4), int32(pos[2] >> 4)})
	if !ok {
		return
	}

	c.SetBlock(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0, rid)
}

// GetNearbyBBoxes returns a list of block bounding boxes that are within the given bounding box - which is usually
// the player's bounding box.
func (p *Player) GetNearbyBBoxes(aabb cube.BBox) []cube.BBox {
	grown := aabb.Grow(0.5)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))

	// A prediction of one BBox per block, plus an additional 2, in case
	var bboxList []cube.BBox
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				block := p.Block(pos)
				boxes := utils.BlockBoxes(p.Block(pos), pos, p.SurroundingBlocks(pos))

				for _, box := range boxes {
					b := box.Translate(pos.Vec3())
					if !b.IntersectsWith(aabb) || utils.CanPassBlock(block) {
						continue
					}

					bboxList = append(bboxList, b)
				}
			}
		}
	}
	return bboxList
}

// GetNearbyBlocks returns a list of blocks that are within the given bounding box.
func (p *Player) GetNearbyBlocks(aabb cube.BBox) map[cube.Pos]world.Block {
	grown := aabb.Grow(0.5)
	min, max := grown.Min(), grown.Max()
	minX, minY, minZ := int(math32.Floor(min[0])), int(math32.Floor(min[1])), int(math32.Floor(min[2]))
	maxX, maxY, maxZ := int(math32.Ceil(max[0])), int(math32.Ceil(max[1])), int(math32.Ceil(max[2]))

	// A prediction of one BBox per block, plus an additional 2, in case
	blocks := make(map[cube.Pos]world.Block)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				blocks[pos] = p.Block(pos)
			}
		}
	}

	return blocks
}

// networkClientBreaksBlock is called when the client has data in PlayerAuthInput
// that is has broken a block.
func (p *Player) networkClientBreaksBlock(pos protocol.BlockPos) {
	// Get the position of the block the client is breaking
	spos := utils.BlockToCubePos(pos)
	b, _ := world.BlockByRuntimeID(air)

	p.SetBlock(spos, b)
}

// cleanChunks filters out any chunks that are out of the player's view, and returns a value of
// how many chunks were cleaned
func (p *Player) cleanChunks() {
	p.chunkMu.Lock()
	defer p.chunkMu.Unlock()

	loc := p.mInfo.ServerPosition
	activePos := protocol.ChunkPos{int32(math32.Floor(loc[0])) >> 4, int32(math32.Floor(loc[2])) >> 4}

	// Delete from any chunks that are out of the player's view.
	for pos := range p.chunks {
		diffX, diffZ := pos[0]-activePos[0], pos[1]-activePos[1]
		dist := math32.Sqrt(float32(diffX*diffX) + float32(diffZ*diffZ))

		// If the distance is within the player's chunk view, leave it alone.
		if int32(dist) <= p.chunkRadius {
			continue
		}

		// The chunks are out of the player's view, so delete it.
		delete(p.chunks, pos)
	}
}

// clearAllChunks clears all chunks from the player's chunk map and unsubscribes from any cached chunks.
func (p *Player) clearAllChunks() {
	p.chunkMu.Lock()
	defer p.chunkMu.Unlock()

	for k := range p.chunks {
		delete(p.chunks, k)
	}
}
