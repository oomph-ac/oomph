package player

import (
	"github.com/df-mc/dragonfly/server/world"
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

// GetTargetBlock checks if the user's ray interesects with any blocks, which may prevent
// them from interacting with entities for combat.
func (p *Player) GetTargetBlock(ray mgl32.Vec3, pos mgl32.Vec3, dist float32) (world.Block, float32) {
	blockMap := make(map[cube.Pos]world.Block)
	for i := float32(0); i <= dist; i += 0.2 {
		bpos := cube.PosFromVec3(pos.Add(ray.Mul(i)))
		blockMap[bpos] = p.World().GetBlock(bpos)
	}

	for bpos, b := range blockMap {
		bbs := utils.BlockBoxes(b, bpos, p.World())
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
