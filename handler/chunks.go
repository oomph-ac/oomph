package handler

import (
	_ "unsafe"

	"bytes"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDChunks = "oomph:chunks"

// noinspection ALL
//
//go:linkname world_finaliseBlockRegistry github.com/df-mc/dragonfly/server/world.finaliseBlockRegistry
func world_finaliseBlockRegistry()

func init() {
	world_finaliseBlockRegistry()
}

type ChunksHandler struct {
	Radius               int32
	BlockPlacements      []BlockPlacement
	BroadcastGhostBlocks bool

	TicksInLoadedChunk int64
	InLoadedChunk      bool

	placedBlocks       map[cube.Pos]df_world.Block
	breakingBlockPos   *protocol.BlockPos
	prevPlaceRequest   *protocol.UseItemTransactionData
	lastPlaceBlockTick int64
	ticked             bool
	initalized         bool
}

type BlockPlacement struct {
	Position cube.Pos
	Block    df_world.Block

	ClickedBlock df_world.Block
	RawData      protocol.UseItemTransactionData

	Sneaking bool
}

func NewChunksHandler() *ChunksHandler {
	return &ChunksHandler{
		Radius:               512,
		BroadcastGhostBlocks: true,
		placedBlocks:         map[cube.Pos]df_world.Block{},
	}
}

func (h *ChunksHandler) ID() string {
	return HandlerIDChunks
}

func (h *ChunksHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		h.tryPlaceBlock(p, pk)
	case *packet.PlayerAuthInput:
		if !h.initalized {
			h.Radius = int32(p.GameDat.ChunkRadius) + 4
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
					//h.breakingBlockPos = nil
				}
			}
		}

		p.World.CleanChunks(h.Radius, chunkPos)
		h.InLoadedChunk = (p.World.GetChunk(chunkPos) != nil)
		if h.InLoadedChunk {
			h.TicksInLoadedChunk++
		} else {
			h.TicksInLoadedChunk = 0
		}

		h.ticked = true
	case *packet.RequestChunkRadius:
		h.Radius = pk.ChunkRadius + 4
	}

	return true
}

func (h *ChunksHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.ChunkRadiusUpdated:
		// We have an increased chunk radius here just in case.
		h.Radius = pk.ChunkRadius + 4
	case *packet.UpdateBlock:
		b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			p.Log().Errorf("unable to find block with runtime ID %v", pk.NewBlockRuntimeID)
			b = block.Air{}
		}

		pos := cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
		isAir := utils.BlockName(b) == "minecraft:air"

		ghBlock := false
		if placed, ok := h.placedBlocks[pos]; ok {
			// If we placed a block at that position but the server declines it, we want to mark it as a ghost block.
			// As sometimes, the client has a race condition where the block won't properly update on its end.
			if isAir {
				p.World.MarkGhostBlock(pos, placed)
				ghBlock = true
			} else {
				// If the block we placed isn't air, we want to unmark it as a ghost block. Blocks we haven't placed don't
				// need to be unmarked as ghost blocks. They should properly update on the client-side.
				p.World.UnmarkGhostBlock(pos)
			}
		}

		// This is done because some server-sided plugins may mess up with oomph's compatibility with being able
		// to account for ghost blocks properly (e.g - BlockLagFix).
		if !h.BroadcastGhostBlocks && ghBlock && p.ClientFrame-h.lastPlaceBlockTick <= 20 {
			return false
		}
		delete(h.placedBlocks, pos)

		// If the client is on semi-authoritative mode (deprecated for movement), send an acknowledgement
		// to the client to know when the block is updated on the client-side world.
		if p.MovementMode != player.AuthorityModeComplete {
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
				ack.AckWorldSetBlock,
				pos,
				b,
			))
		} else {
			// On full authoritative mode, we want to update the block ASAP to prevent ghost blocks, etc.
			ack.DirectCall(ack.AckWorldSetBlock, p, pos, b)
		}
	case *packet.LevelChunk:
		// Check if this LevelChunk packet is compatiable with oomph's handling.
		if pk.SubChunkCount == protocol.SubChunkRequestModeLimited || pk.SubChunkCount == protocol.SubChunkRequestModeLimitless {
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

// tryPlaceBlock attempts to place a block in Oomph's lag-compensated World. It accounts for ghost blocks
// as well.
func (h *ChunksHandler) tryPlaceBlock(p *player.Player, pk *packet.InventoryTransaction) {
	// If the world has ghost blocks, we want to account for the fact that the client may try to stack
	// blocks on top of those ghost blocks before recieving an update from the server.
	if p.World.HasGhostBlocks() {
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
	if dat.HeldItem.Stack.BlockRuntimeID <= 0 {
		return
	}

	b, ok := df_world.BlockByRuntimeID(uint32(dat.HeldItem.Stack.BlockRuntimeID))
	if !ok {
		return
	}

	if h.prevPlaceRequest != nil && dat.BlockRuntimeID == h.prevPlaceRequest.BlockRuntimeID && dat.BlockFace == h.prevPlaceRequest.BlockFace &&
		dat.BlockPosition == h.prevPlaceRequest.BlockPosition && dat.HotBarSlot == h.prevPlaceRequest.HotBarSlot &&
		dat.Position == h.prevPlaceRequest.Position && dat.ClickedPosition == h.prevPlaceRequest.ClickedPosition {
		return
	}

	defer func() {
		h.prevPlaceRequest = dat
	}()

	// Find the replace position of the block. This will be used if the block at the current position
	// is replacable (e.g: water, lava, air).
	replacePos := utils.BlockToCubePos(dat.BlockPosition)
	fb := p.World.GetBlock(replacePos)
	clickedBlockIsGhost := p.World.IsGhostBlock(replacePos)

	// If the block at the position is not replacable, we want to place the block on the side of the block.
	if replaceable, ok := fb.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
		replacePos = replacePos.Side(cube.Face(dat.BlockFace))
	}

	// Make a list of BBoxes the block will occupy.
	boxes := utils.BlockBoxes(b, replacePos, p.World)

	// Get the player's AABB and translate it to the position of the player. Then check if it intersects
	// with any of the boxes the block will occupy. If it does, we don't want to place the block.
	movHandler := p.Handler(HandlerIDMovement).(*MovementHandler)
	if cube.AnyIntersections(boxes, movHandler.BoundingBox()) {
		return
	}

	// Check if any entity is in the way of the block being placed.
	entHandler := p.Handler(HandlerIDEntities).(*EntitiesHandler)
	for _, e := range entHandler.Entities {
		if cube.AnyIntersections(boxes, e.Box(e.Position)) {
			return
		}
	}

	// The handler's BlockPlacements makes it easy for any detection attempting to handle block placements
	// to know what blocks are being placed and where without double processing. This is mainly used for
	// Scaffold detections.
	mDat := p.Handler(HandlerIDMovement).(*MovementHandler)
	h.BlockPlacements = append(h.BlockPlacements, BlockPlacement{
		Position: replacePos,
		Block:    b,

		ClickedBlock: fb,
		RawData:      *dat,

		Sneaking: mDat.SneakKeyPressed,
	})

	h.lastPlaceBlockTick = p.ClientFrame
	if clickedBlockIsGhost {
		p.World.MarkGhostBlock(replacePos, fb)
		return
	}
	p.World.SetBlock(replacePos, b)
	h.placedBlocks[replacePos] = b
}

// noinspection ALL
//
//go:linkname chunk_subChunkDecode github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func chunk_subChunkDecode(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
