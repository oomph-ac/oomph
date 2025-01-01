package acknowledgement

import (
	"bytes"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ChunkUpdate is an acknowledgment that runs when a player recieves a LevelChunk packet from the server.
type ChunkUpdate struct {
	mPlayer *player.Player
	pk      *packet.LevelChunk
}

func NewChunkUpdateACK(p *player.Player, pk *packet.LevelChunk) *ChunkUpdate {
	return &ChunkUpdate{mPlayer: p, pk: pk}
}

func (ack *ChunkUpdate) Run() {
	ack.mPlayer.World.ExemptChunk(ack.pk.Position)
	oworld.Cache(ack.mPlayer.World, ack.pk)
}

// SubChunkUpdate is an acknowledgment that runs when a player recievs a SubChunk packet.
type SubChunkUpdate struct {
	mPlayer *player.Player
	pk      *packet.SubChunk
}

func NewSubChunkUpdateACK(p *player.Player, pk *packet.SubChunk) *SubChunkUpdate {
	return &SubChunkUpdate{mPlayer: p, pk: pk}
}

func (ack *SubChunkUpdate) Run() {
	if ack.pk.CacheEnabled {
		panic(oerror.New("subchunk caching not supported on oomph"))
	}
	var newChunks = map[protocol.ChunkPos]*chunk.Chunk{}

	for _, entry := range ack.pk.SubChunkEntries {
		if entry.Result != protocol.SubChunkResultSuccess && entry.Result != protocol.SubChunkResultSuccessAllAir {
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "unhandled subchunk result %d @ %v", entry.Result, ack.pk.Position)
			continue
		}

		chunkPos := protocol.ChunkPos{
			ack.pk.Position[0] + int32(entry.Offset[0]),
			ack.pk.Position[2] + int32(entry.Offset[2]),
		}

		var c *chunk.Chunk
		if entry.Result == protocol.SubChunkResultSuccessAllAir {
			c = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
			newChunks[chunkPos] = c
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "all air at %v", chunkPos)
		} else if new, ok := newChunks[chunkPos]; ok {
			c = new
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "reusing chunk in map %v", chunkPos)
		} else if existing := ack.mPlayer.World.GetChunk(chunkPos); existing != nil {
			// We assume that the existing chunk is not cached because the cache does not support SubChunks for the time being.
			c = existing.(*chunk.Chunk)
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "using existing chunk %v", chunkPos)
		} else {
			c = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
			newChunks[chunkPos] = c
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "new chunk at %v", chunkPos)
		}

		buf := internal.BufferPool.Get().(*bytes.Buffer)
		defer internal.BufferPool.Put(buf)
		buf.Reset()
		buf.Write(entry.RawPayload)

		if entry.Result != protocol.SubChunkResultSuccessAllAir {
			var index byte
			sub, err := utils.DecodeSubChunk(buf, c, &index, chunk.NetworkEncoding)
			if err != nil {
				panic(err)
			}
			c.Sub()[index] = sub
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "decoded subchunk %d at %v", index, chunkPos)
		}

	}

	for pos, newC := range newChunks {
		ack.mPlayer.World.AddChunk(pos, newC)
		ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "(sub) added chunk at %v", pos)
	}
}
