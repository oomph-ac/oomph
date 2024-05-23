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
	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDChunks = "oomph:chunks"

type ChunksHandler struct {
	ChunkRadius     int32
	InLoadedChunk   bool
	BlockPlacements []BlockPlacement

	placedBlocks     map[cube.Pos]df_world.Block
	breakingBlockPos *protocol.BlockPos
	ticked           bool
	initalized       bool
}

type BlockPlacement struct {
	Position cube.Pos
	Block    df_world.Block

	ClickedBlock df_world.Block
	RawData      protocol.UseItemTransactionData
}

func NewChunksHandler() *ChunksHandler {
	return &ChunksHandler{
		ChunkRadius:  512,
		placedBlocks: map[cube.Pos]df_world.Block{},
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
	case *packet.InventoryTransaction:
		h.tryPlaceBlock(p, pk, false)

		// If there are ghost blocks in the world, account for scenarios where the player will
		// place a block on top of a ghost block, etc.
		if p.World.HasGhostBlocks() {
			h.tryPlaceBlock(p, pk, true)
		}
	case *packet.PlayerAuthInput:
		if !h.initalized {
			h.ChunkRadius = int32(p.GameDat.ChunkRadius) + 4
			h.initalized = true
		}

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

					p.World.SetBlock(cube.Pos{
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

					h.breakingBlockPos = &action.BlockPos
				case protocol.PlayerActionAbortBreak:
					h.breakingBlockPos = nil
				case protocol.PlayerActionStopBreak:
					if h.breakingBlockPos == nil {
						continue
					}

					p.World.SetBlock(cube.Pos{
						int(h.breakingBlockPos.X()),
						int(h.breakingBlockPos.Y()),
						int(h.breakingBlockPos.Z()),
					}, block.Air{})
					h.breakingBlockPos = nil
				}
			}
		}

		p.World.CleanChunks(h.ChunkRadius, chunkPos)
		h.InLoadedChunk = (p.World.GetChunk(chunkPos) != nil)
		h.ticked = true
	case *packet.RequestChunkRadius:
		h.ChunkRadius = pk.ChunkRadius + 4
	}

	return true
}

func (h *ChunksHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.ChunkRadiusUpdated:
		h.ChunkRadius = pk.ChunkRadius + 4
	case *packet.UpdateBlock:
		b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			p.Log().Errorf("unable to find block with runtime ID %v", pk.NewBlockRuntimeID)
			b = block.Air{}
		}

		pos := cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
		isAir := utils.BlockName(b) == "minecraft:air"

		if placed, ok := h.placedBlocks[pos]; ok && isAir {
			p.World.MarkGhostBlock(pos, placed)
		} else if !ok {
			p.World.UnmarkGhostBlock(pos)
		}
		delete(h.placedBlocks, pos)

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckWorldSetBlock,
			pos,
			b,
		))
	case *packet.LevelChunk:
		// Check if this LevelChunk packet is compatiable with oomph's handling.
		if pk.SubChunkCount == protocol.SubChunkRequestModeLimited || pk.SubChunkCount == protocol.SubChunkRequestModeLimitless {
			return true
		}

		if p.MovementMode == player.AuthorityModeNone {
			return true
		}

		// NOTE: The reason we have to make a clone of the packet here is because multiversion implementations will downgrade the packet
		// and Oomph, instead of using the regular chunk packet sent by the server, will use the one modified by the multiversion implementation
		// since it is a pointer.
		cpk := *pk
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckWorldUpdateChunks,
			&cpk,
		))
	case *packet.SubChunk:
		if p.MovementMode == player.AuthorityModeNone {
			return true
		}

		// NOTE: The reason we have to make a clone of the packet here is because multiversion implementations will downgrade the packet
		// and Oomph, instead of using the regular chunk packet sent by the server, will use the one modified by the multiversion implementation
		// since it is a pointer.
		cpk := *pk
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckWorldUpdateChunks,
			&cpk,
		))
	}

	return true
}

func (h *ChunksHandler) OnTick(p *player.Player) {
}

func (h *ChunksHandler) Defer() {
	if h.ticked {
		h.BlockPlacements = []BlockPlacement{}
		h.ticked = false
	}
}

func (h *ChunksHandler) tryPlaceBlock(p *player.Player, pk *packet.InventoryTransaction, ghost bool) {
	if ghost {
		p.World.SearchWithGhost(true)
		defer p.World.SearchWithGhost(false)
	}

	dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return
	}

	// Validate action type.
	if dat.ActionType != protocol.UseItemActionClickBlock {
		return
	}

	// No item in hand.
	if dat.HeldItem.Stack.NetworkID == 0 {
		return
	}

	// BlockRuntimeIDs should be positive.
	if dat.HeldItem.Stack.BlockRuntimeID < 0 {
		return
	}

	b, ok := df_world.BlockByRuntimeID(uint32(dat.HeldItem.Stack.BlockRuntimeID))
	if !ok {
		return
	}

	// Find the replace position of the block. This will be used if the block at the current position
	// is replacable (e.g: water, lava, air).
	replacePos := utils.BlockToCubePos(dat.BlockPosition)
	fb := p.World.GetBlock(replacePos)

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
			return
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
		return
	}

	entHandler := p.Handler(HandlerIDEntities).(*EntitiesHandler)
	for _, e := range entHandler.Entities {
		if cube.AnyIntersections(boxes, e.Box(e.Position)) {
			return
		}
	}

	if ghost {
		p.World.MarkGhostBlock(replacePos, b)
		return
	}

	h.BlockPlacements = append(h.BlockPlacements, BlockPlacement{
		Position: replacePos,
		Block:    b,

		ClickedBlock: fb,
		RawData:      *dat,
	})
	p.World.SetBlock(replacePos, b)
	h.placedBlocks[replacePos] = b
}

// noinspection ALL
//
//go:linkname chunk_subChunkDecode github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func chunk_subChunkDecode(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
