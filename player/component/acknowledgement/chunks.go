package acknowledgement

import (
	"bytes"
	"fmt"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/player"
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
	cInfo, err := oworld.CacheChunk(ack.pk)
	if err != nil {
		ack.mPlayer.Disconnect(fmt.Sprintf(game.ErrorInternalDecodeChunk, err))
		return
	}
	ack.mPlayer.World().AddChunk(ack.pk.Position, cInfo)
	ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "added chunk at %v", ack.pk.Position)
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

	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()
	var bufUsed bool

	newChunks := make(map[protocol.ChunkPos]*chunk.Chunk)
	for _, entry := range ack.pk.SubChunkEntries {
		chunkPos := protocol.ChunkPos{
			ack.pk.Position[0] + int32(entry.Offset[0]),
			ack.pk.Position[2] + int32(entry.Offset[2]),
		}
		var ch *chunk.Chunk
		if new, ok := newChunks[chunkPos]; ok {
			ch = new
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "reusing chunk in map %v", chunkPos)
		} else if existing := ack.mPlayer.World().GetChunk(chunkPos); existing != nil {
			// We assume that the existing chunk is not cached because the cache does not support SubChunks for the time being.
			ch = existing
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "using existing chunk %v", chunkPos)
		} else {
			dimension, ok := world.DimensionByID(int(ack.pk.Dimension))
			if !ok {
				dimension = world.Overworld
			}
			ch = chunk.New(oworld.AirRuntimeID, dimension.Range())
			newChunks[chunkPos] = ch
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "new chunk at %v", chunkPos)
		}

		switch entry.Result {
		case protocol.SubChunkResultSuccess:
			if bufUsed {
				buf.Reset()
			}
			bufUsed = true
			buf.Write(entry.RawPayload)

			cachedSub, err := oworld.CacheSubChunk(buf, ch, chunkPos)
			if err != nil {
				ack.mPlayer.Disconnect(fmt.Sprintf(game.ErrorInternalDecodeChunk, err))
				continue
			}
			ch.Sub()[cachedSub.Layer()] = cachedSub.SubChunk()
			ack.mPlayer.World().AddSubChunk(chunkPos, cachedSub.Hash())
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "cached subchunk %d at %v", cachedSub.Layer(), chunkPos)
		case protocol.SubChunkResultSuccessAllAir:
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "all-air chunk at %v", chunkPos)
		default:
			ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "no subchunk data for %v (result=%d)", chunkPos, entry.Result)
			continue
		}
	}

	for pos, newChunk := range newChunks {
		ack.mPlayer.World().AddChunk(pos, oworld.ChunkInfo{Chunk: newChunk, Cached: false})
		ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "(sub) added chunk at %v", pos)
	}
}
