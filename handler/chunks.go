package handler

import (
	"fmt"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDChunks = "oomph:chunks"

type ChunksHandler struct {
	World       *world.World
	ChunkRadius int32
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
		h.World.CleanChunks(h.ChunkRadius, protocol.ChunkPos{
			int32(math32.Floor(pk.Position.X())) >> 4,
			int32(math32.Floor(pk.Position.Z())) >> 4,
		})

		n, _ := h.World.GetBlock(pk.Position.Sub(mgl32.Vec3{0, 2.62, 0})).EncodeBlock()
		p.Message(fmt.Sprintf("%s (%v) [%v]", n, p.ClientFrame, h.ChunkRadius))
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
	}

	return true
}

func (h *ChunksHandler) OnTick(p *player.Player) {
}

func (h *ChunksHandler) Defer() {
}
