package event

import (
	"encoding/base64"
	"encoding/json"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/oerror"
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
	dat := map[string]interface{}{}
	dat["EvTime"] = e.EvTime
	dat["Position"] = e.Position
	dat["Range"] = e.Range

	// Encode the chunk.
	serialized := chunk.Encode(e.Chunk, chunk.DiskEncoding)
	subs := make([]string, len(serialized.SubChunks))
	for i, sub := range serialized.SubChunks {
		subs[i] = base64.StdEncoding.EncodeToString(sub)
	}

	dat["SubChunks"] = subs
	dat["Biomes"] = base64.StdEncoding.EncodeToString(serialized.Biomes)

	enc, err := json.Marshal(dat)
	if err != nil {
		panic(oerror.New("error encoding event: " + err.Error()))
	}

	return append([]byte{e.ID()}, enc...)
}
