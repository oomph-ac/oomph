package handler

import (
	_ "unsafe"

	"bytes"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDChunks = "oomph:chunks"

type ChunksHandler struct {
	World         *world.World
	ChunkRadius   int32
	InLoadedChunk bool

	breakingBlockPos *protocol.BlockPos
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
	if p.MovementMode == player.AuthorityModeNone {
		return true
	}

	switch pk := pk.(type) {
	case *packet.TickSync:
		if p.ServerConn() == nil {
			h.ChunkRadius = p.Conn().GameData().ChunkRadius
		} else {
			h.ChunkRadius = p.ServerConn().GameData().ChunkRadius
		}
	case *packet.InventoryTransaction:
		dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
		if !ok {
			return true
		}

		i, ok := df_world.ItemByRuntimeID(dat.HeldItem.Stack.NetworkID, int16(dat.HeldItem.Stack.MetadataValue))
		if !ok {
			return true
		}

		// Determine if the item can be placed as a block.
		b, ok := i.(df_world.Block)
		if !ok {
			return true
		}

		// Find the replace position of the block. This will be used if the block at the current position
		// is replacable (e.g: water, lava, air).
		replacePos := utils.BlockToCubePos(dat.BlockPosition)
		fb := h.World.GetBlock(replacePos)

		// If the block at the position is not replacable, we want to place the block on the side of the block.
		if replaceable, ok := fb.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
			replacePos = replacePos.Side(cube.Face(dat.BlockFace))
		}

		// Make a list of BBoxes the block will occupy.
		bx := b.Model().BBox(df_cube.Pos(replacePos), nil)
		boxes := make([]cube.BBox, 0)
		for _, bxx := range bx {
			// Don't continue if the block isn't 1x1x1.
			// TODO: Implement placements for these blocks properly.
			if bxx.Width() != 1 || bxx.Height() != 1 || bxx.Length() != 1 {
				return true
			}

			boxes = append(boxes, game.DFBoxToCubeBox(bxx).Translate(mgl32.Vec3{
				float32(replacePos.X()),
				float32(replacePos.Y()),
				float32(replacePos.Z()),
			}))
		}

		// Get the player's AABB and translate it to the position of the player. Then check if it intersects
		// with any of the boxes the block will occupy. If it does, we don't want to place the block.
		movHandler := p.Handler(HandlerIDMovement).(*MovementHandler)
		if cube.AnyIntersections(boxes, movHandler.BoundingBox()) {
			return true
		}

		entHandler := p.Handler(HandlerIDEntities).(*EntitiesHandler)
		for _, e := range entHandler.Entities {
			if cube.AnyIntersections(boxes, e.Box(e.Position)) {
				return true
			}
		}

		// Set the block in the world.
		h.World.SetBlock(replacePos, b)
		return true
	case *packet.PlayerAuthInput:
		chunkPos := protocol.ChunkPos{
			int32(math32.Floor(pk.Position.X())) >> 4,
			int32(math32.Floor(pk.Position.Z())) >> 4,
		}

		if utils.HasFlag(pk.InputData, packet.InputFlagPerformBlockActions) {
			for _, action := range pk.BlockActions {
				switch action.Action {
				case protocol.PlayerActionPredictDestroyBlock:
					if p.ServerConn() == nil {
						continue
					}
					
					if !p.ServerConn().GameData().PlayerMovementSettings.ServerAuthoritativeBlockBreaking {
						continue
					}

					h.World.SetBlock(cube.Pos{
						int(action.BlockPos.X()),
						int(action.BlockPos.Y()),
						int(action.BlockPos.Z()),
					}, block.Air{})
				case protocol.PlayerActionStartBreak:
					if h.breakingBlockPos != nil {
						continue
					}

					h.breakingBlockPos = &action.BlockPos
				case protocol.PlayerActionCrackBreak:
					if h.breakingBlockPos == nil {
						continue
					}

					if *h.breakingBlockPos == action.BlockPos {
						continue
					}

					panic(oerror.New("invalid block action recieved [1]"))
				case protocol.PlayerActionAbortBreak:
					h.breakingBlockPos = nil
				case protocol.PlayerActionStopBreak:
					if h.breakingBlockPos == nil {
						continue
					}

					h.World.SetBlock(cube.Pos{
						int(h.breakingBlockPos.X()),
						int(h.breakingBlockPos.Y()),
						int(h.breakingBlockPos.Z()),
					}, block.Air{})
					h.breakingBlockPos = nil
				}
			}
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
