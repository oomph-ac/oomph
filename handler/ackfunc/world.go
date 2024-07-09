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
	p.Dbg.Notify(player.DebugModeChunks, true, "set block at %v to %s", opts[0].(cube.Pos), utils.BlockName(opts[1].(world.Block)))
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

		p.Dbg.Notify(player.DebugModeChunks, true, "received chunk update at %v", cpk.Position)
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
			p.Dbg.Notify(player.DebugModeChunks, true, "unhandled subchunk result %d @ %v", entry.Result, spk.Position)
			continue
		}

		chunkPos := protocol.ChunkPos{
			spk.Position[0] + int32(entry.Offset[0]),
			spk.Position[2] + int32(entry.Offset[2]),
		}

		var c *chunk.Chunk
		if entry.Result == protocol.SubChunkResultSuccessAllAir {
			c = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
			newChunks[chunkPos] = c
			p.Dbg.Notify(player.DebugModeChunks, true, "all air at %v", chunkPos)
		} else if new, ok := newChunks[chunkPos]; ok {
			c = new
			p.Dbg.Notify(player.DebugModeChunks, true, "reusing chunk in map %v", chunkPos)
		} else if existing := p.World.GetChunk(chunkPos); existing != nil {
			c = existing
			p.Dbg.Notify(player.DebugModeChunks, true, "using existing chunk %v", chunkPos)
		} else {
			c = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
			newChunks[chunkPos] = c
			p.Dbg.Notify(player.DebugModeChunks, true, "new chunk at %v", chunkPos)
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
			p.Dbg.Notify(player.DebugModeChunks, true, "decoded subchunk %d at %v", index, chunkPos)
		}

		internal.BufferPool.Put(buf)
	}

	for pos, newC := range newChunks {
		p.World.AddChunk(pos, newC)
		p.Dbg.Notify(player.DebugModeChunks, true, "(sub) added chunk at %v", pos)
	}
}
