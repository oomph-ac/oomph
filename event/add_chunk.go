package event

import (
	"bytes"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type AddChunkEvent struct {
	NopEvent

	Chunk    *chunk.Chunk
	Range    cube.Range
	Position protocol.ChunkPos
}

func (e AddChunkEvent) ID() byte {
	return EventIDAddChunk
}

func (e AddChunkEvent) Encode() []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer internal.BufferPool.Put(buf)

	WriteEventHeader(e, buf)
	utils.WriteLInt32(buf, e.Position.X())
	utils.WriteLInt32(buf, e.Position.Z())

	utils.WriteLInt64(buf, int64(e.Range[0]))
	utils.WriteLInt64(buf, int64(e.Range[1]))

	// Encode the chunk.
	serialized := chunk.Encode(e.Chunk, chunk.DiskEncoding)

	utils.WriteLInt32(buf, int32(len(serialized.SubChunks)))
	for _, sub := range serialized.SubChunks {
		utils.WriteLInt32(buf, int32(len(sub)))
		buf.Write(sub)
	}

	utils.WriteLInt32(buf, int32(len(serialized.Biomes)))
	buf.Write(serialized.Biomes)

	return buf.Bytes()
}
