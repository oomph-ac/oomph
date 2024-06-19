package ackfunc

import (
	"bytes"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// OPTS: cube.Pos, world.Block
func WorldSetBlock(p *player.Player, opts ...interface{}) {
	p.World.SetBlock(opts[0].(cube.Pos), opts[1].(world.Block))
}

// OPTS: packet.Packet
func WorldUpdateChunks(p *player.Player, opts ...interface{}) {
	pk := opts[0].(packet.Packet)

	if cpk, ok := pk.(*packet.LevelChunk); ok {
		c, err := chunk.NetworkDecode(oworld.AirRuntimeID, cpk.RawPayload, int(cpk.SubChunkCount), world.Overworld.Range())
		if err != nil {
			c = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
		}
		c.Compact()

		p.World.AddChunk(cpk.Position, c)
		return
	}

	spk, ok := pk.(*packet.SubChunk)
	if !ok {
		panic(oerror.New("unexpected packet in chunk update ack: %T", spk))
	}

	if spk.CacheEnabled {
		panic(oerror.New("subchunk caching not supported on oomph"))
	}
	var newChunks = map[protocol.ChunkPos]*chunk.Chunk{}

	for _, entry := range spk.SubChunkEntries {
		if entry.Result != protocol.SubChunkResultSuccess && entry.Result != protocol.SubChunkResultSuccessAllAir {
			continue
		}

		chunkPos := protocol.ChunkPos{
			spk.Position[0] + int32(entry.Offset[0]),
			spk.Position[2] + int32(entry.Offset[2]),
		}

		var c *chunk.Chunk
		if entry.Result == protocol.SubChunkResultSuccessAllAir {
			c = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
		} else if existing := p.World.GetChunk(chunkPos); existing != nil {
			c = existing
		} else if new, ok := newChunks[chunkPos]; ok {
			c = new
		} else {
			c = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
			newChunks[chunkPos] = c
		}

		buf := internal.BufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		buf.Write(entry.RawPayload)

		if entry.Result != protocol.SubChunkResultSuccessAllAir {
			var index byte
			sub, err := utils.DecodeSubChunk(buf, c, &index, chunk.NetworkEncoding)
			if err != nil {
				panic(err)
			}
			c.Sub()[index] = sub
		}


		internal.BufferPool.Put(buf)
	}

	for pos, newC := range newChunks {
		p.World.AddChunk(pos, newC)
	}
}
