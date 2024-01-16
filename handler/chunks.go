package handler

import (
	_ "unsafe"

	"bytes"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDChunks = "oomph:chunks"

type ChunksHandler struct {
	World       *world.World
	ChunkRadius int32

	InLoadedChunk bool
}

func NewChunksHandler() *ChunksHandler {
	return &ChunksHandler{
		World:       world.NewWorld(),
		ChunkRadius: -1,
	}
}

func (h *ChunksHandler) ID() string {
	return HandlerIDChunks
}

func (h *ChunksHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.TickSync:
		h.ChunkRadius = p.ServerConn().GameData().ChunkRadius
	case *packet.PlayerAuthInput:
		// TODO: Use server position on full authority mode.
		chunkPos := protocol.ChunkPos{
			int32(math32.Floor(pk.Position.X())) >> 4,
			int32(math32.Floor(pk.Position.Z())) >> 4,
		}

		h.World.CleanChunks(h.ChunkRadius, chunkPos)
		h.InLoadedChunk = (h.World.GetChunk(chunkPos, true) != nil)
	case *packet.RequestChunkRadius:
		h.ChunkRadius = pk.ChunkRadius
	}

	return true
}

func (h *ChunksHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.ChunkRadiusUpdated:
		h.ChunkRadius = pk.ChunkRadius
	case *packet.UpdateBlock:
		b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			p.Log().Errorf("unable to find block with runtime ID %v", pk.NewBlockRuntimeID)
			b = block.Air{}
		}

		h.World.SetBlock(cube.Pos{
			int(pk.Position.X()),
			int(pk.Position.Y()),
			int(pk.Position.Z()),
		}, b)
	case *packet.LevelChunk:
		// Check if this LevelChunk packet is compatiable with oomph's handling.
		if pk.SubChunkCount == protocol.SubChunkRequestModeLimited || pk.SubChunkCount == protocol.SubChunkRequestModeLimitless {
			return true
		}

		if p.MovementMode == player.AuthorityModeNone {
			return true
		}

		// Decode the chunk data, and remove any uneccessary data via. Compact().
		c, err := chunk.NetworkDecode(world.AirRuntimeID, pk.RawPayload, int(pk.SubChunkCount), df_world.Overworld.Range())
		if err != nil {
			c = chunk.New(world.AirRuntimeID, df_world.Overworld.Range())
		}

		c.Compact()
		world.InsertToCache(h.World, c, pk.Position)
	case *packet.SubChunk:
		if pk.CacheEnabled {
			panic(oerror.New("subchunk caching not supported on oomph"))
		}

		var newChunks = map[protocol.ChunkPos]*chunk.Chunk{}

		for _, entry := range pk.SubChunkEntries {
			// Do not handle sub-chunk responses that returned an error.
			if entry.Result != protocol.SubChunkResultSuccess {
				continue
			}

			chunkPos := protocol.ChunkPos{
				pk.Position[0] + int32(entry.Offset[0]),
				pk.Position[2] + int32(entry.Offset[2]),
			}

			var cached *world.CachedChunk
			var c *chunk.Chunk

			if found := h.World.GetChunk(chunkPos, true); found != nil {
				cached = found
				c = found.Chunk
			} else {
				if new, ok := newChunks[chunkPos]; !ok {
					c = chunk.New(world.AirRuntimeID, dimensionFromNetworkID(pk.Dimension).Range())
				} else {
					c = new
				}
			}

			var index byte
			sub, err := chunk_subChunkDecode(bytes.NewBuffer(entry.RawPayload), c, &index, chunk.NetworkEncoding)
			if err != nil {
				panic(err)
			}

			if cached != nil {
				cached.InsertSubChunk(h.World, sub, index)
				return true
			}

			c.Sub()[index] = sub
			newChunks[chunkPos] = c
		}

		for pos, newC := range newChunks {
			world.InsertToCache(h.World, newC, pos)
		}
	}

	return true
}

func (h *ChunksHandler) OnTick(p *player.Player) {
}

func (h *ChunksHandler) Defer() {
}

// dimensionFromNetworkID returns a world.Dimension from the network id.
func dimensionFromNetworkID(id int32) df_world.Dimension {
	if id == 1 {
		return df_world.Nether
	}
	if id == 2 {
		return df_world.End
	}
	return df_world.Overworld
}

// noinspection ALL
//
//go:linkname chunk_subChunkDecode github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func chunk_subChunkDecode(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
