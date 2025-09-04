package world

import (
	"bytes"

	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

func ChunkBlockEntities(c *chunk.Chunk, buf *bytes.Buffer) map[df_cube.Pos]world.Block {
	dec := nbt.NewDecoderWithEncoding(buf, nbt.NetworkLittleEndian)
	blockNBTs := make(map[df_cube.Pos]world.Block)
	for {
		var decNbt map[string]any
		if err := dec.Decode(&decNbt); err != nil {
			break
		}
		x, okX := decNbt["x"]
		y, okY := decNbt["y"]
		z, okZ := decNbt["z"]
		if !okX || !okY || !okZ {
			continue
		}
		x2, okX2 := x.(int32)
		y2, okY2 := y.(int32)
		z2, okZ2 := z.(int32)
		if !okX2 || !okY2 || !okZ2 {
			continue
		}
		pos := df_cube.Pos{int(x2), int(y2), int(z2)}
		rid := c.Block(uint8(pos[0]), int16(pos[1]), uint8(pos[2]), 0)
		b, ok := world.BlockByRuntimeID(rid)
		if !ok {
			continue
		}
		blockNBT, ok := b.(world.NBTer)
		if !ok {
			continue
		}
		blockNBTs[pos] = blockNBT.DecodeNBT(decNbt).(world.Block)
	}
	return blockNBTs
}
