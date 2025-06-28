package acknowledgement

import (
	"bytes"
	"fmt"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ChunkUpdate is an acknowledgment that runs when a player receives a LevelChunk packet from the server.
type ChunkUpdate struct {
	mPlayer *player.Player
	pk      *packet.LevelChunk
}

func NewChunkUpdateACK(p *player.Player, pk *packet.LevelChunk) *ChunkUpdate {
	return &ChunkUpdate{mPlayer: p, pk: pk}
}

func (ack *ChunkUpdate) Run() {
	if ack.pk.CacheEnabled {
		ack.mPlayer.Disconnect(game.ErrorChunkCacheUnsupported)
		return
	}

	if cached, err := oworld.Cache(ack.pk); err == nil {
		ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "using cached chunk at %v (hash=%s)", ack.pk.Position, cached.Hash())
		ack.mPlayer.World().AddChunk(ack.pk.Position, cached)
		return
	} else {
		ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "failed to cache chunk at %v, will resort to manual insertion", ack.pk.Position)
	}
	c, err := chunk.NetworkDecode(oworld.AirRuntimeID, ack.pk.RawPayload, int(ack.pk.SubChunkCount), world.Overworld.Range())
	if err != nil {
		ack.mPlayer.Disconnect(fmt.Sprintf(game.ErrorInternalDecodeChunk, err))
		return
	}
	ack.mPlayer.World().AddChunk(ack.pk.Position, c)
}

// SubChunkUpdate is an acknowledgment that runs when a player receives a SubChunk packet.
type SubChunkUpdate struct {
	mPlayer *player.Player
	pk      *packet.SubChunk
}

func NewSubChunkUpdateACK(p *player.Player, pk *packet.SubChunk) *SubChunkUpdate {
	return &SubChunkUpdate{mPlayer: p, pk: pk}
}

func (ack *SubChunkUpdate) Run() {
	if ack.pk.CacheEnabled {
		ack.mPlayer.Disconnect(game.ErrorChunkCacheUnsupported)
		return
	}

	newChunks := make(map[protocol.ChunkPos]*chunk.Chunk)
	for _, entry := range ack.pk.SubChunkEntries {
		chunkPos := protocol.ChunkPos{
			ack.pk.Position[0] + int32(entry.Offset[0]),
			ack.pk.Position[2] + int32(entry.Offset[2]),
		}
		if entry.Result != protocol.SubChunkResultSuccess {
			newChunks[chunkPos] = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
			continue
		}

		var ch *chunk.Chunk
		if new, ok := newChunks[chunkPos]; ok {
			ch = new
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "reusing chunk in map %v", chunkPos)
		} else if existing := ack.mPlayer.World().GetChunk(chunkPos); existing != nil {
			// We assume that the existing chunk is not cached because the cache does not support SubChunks for the time being.
			if c, ok := existing.(*chunk.Chunk); ok {
				ch = c
			} else if cached, ok := existing.(*oworld.CachedChunk); ok {
				chunkClone := cached.Chunk()
				ch = &chunkClone
			} else {
				panic(fmt.Sprintf("unknown oworld.ChunkSource in player world (%T)", existing))
			}
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "using existing chunk %v", chunkPos)
		} else {
			ch = chunk.New(oworld.AirRuntimeID, world.Overworld.Range())
			newChunks[chunkPos] = ch
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "new chunk at %v", chunkPos)
		}

		buf := internal.BufferPool.Get().(*bytes.Buffer)
		defer internal.BufferPool.Put(buf)
		buf.Reset()
		buf.Write(entry.RawPayload)

		var index byte
		sub, err := utils.DecodeSubChunk(buf, ch, &index, chunk.NetworkEncoding)
		if err != nil {
			//panic(err)
			ack.mPlayer.Disconnect(fmt.Sprintf(game.ErrorInternalDecodeChunk, err))
			return
		}

		ch.Sub()[index] = sub
		ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "decoded subchunk %d at %v", index, chunkPos)

	}

	for pos, newChunk := range newChunks {
		ack.mPlayer.World().AddChunk(pos, newChunk)
		ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "(sub) added chunk at %v", pos)
	}
}
