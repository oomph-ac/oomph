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
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/oomph-ac/oomph/game"
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
	Radius          int32
	BlockPlacements []BlockPlacement

	TicksInLoadedChunk int64
	InLoadedChunk      bool

	initalInteraction         *protocol.UseItemTransactionData
	initalInteractionAccepted bool

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
		Radius: 512,
	}
}

func (h *ChunksHandler) ID() string {
	return HandlerIDChunks
}

func (h *ChunksHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if !h.validateInteraction(p, pk) {
			return false
		}
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

					p.World.SetBlock(df_cube.Pos{
						int(action.BlockPos.X()),
						int(action.BlockPos.Y()),
						int(action.BlockPos.Z()),
					}, block.Air{}, nil)
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

					p.World.SetBlock(df_cube.Pos{
						int(h.breakingBlockPos.X()),
						int(h.breakingBlockPos.Y()),
						int(h.breakingBlockPos.Z()),
					}, block.Air{}, nil)
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
		pos := cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
		b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			p.Log().Errorf("unable to find block with runtime ID %v", pk.NewBlockRuntimeID)
			b = block.Air{}
		}

		// If the client is on semi-authoritative mode (deprecated for movement), send an acknowledgement
		// to the client to know when the block is updated on the client-side world.
		if p.MovementMode == player.AuthorityModeSemi {
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
				ack.AckWorldSetBlock,
				df_cube.Pos(pos),
				b,
			))
		} else if p.MovementMode == player.AuthorityModeComplete {
			// On full authoritative mode, we want to update the block ASAP, as we want to simulate movement
			// on the most recent server world state.
			ack.Instant(ack.AckWorldSetBlock, p, df_cube.Pos(pos), b)
		}
	case *packet.LevelChunk:
		// Check if this LevelChunk packet is compatiable with oomph's handling.
		if pk.SubChunkCount == protocol.SubChunkRequestModeLimited || pk.SubChunkCount == protocol.SubChunkRequestModeLimitless {
			return true
		}

		if p.MovementMode == player.AuthorityModeSemi {
			// NOTE: The reason we have to make a clone of the packet here is because multiversion implementations will downgrade the packet
			// and Oomph, instead of using the regular chunk packet sent by the server, will use the one modified by the multiversion implementation
			// since it is a pointer.
			cpk := *pk
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
				ack.AckWorldUpdateChunks,
				&cpk,
			))
		} else if p.MovementMode == player.AuthorityModeComplete {
			ack.Instant(ack.AckWorldUpdateChunks, p, pk)
		}
	case *packet.SubChunk:
		if p.MovementMode == player.AuthorityModeSemi {
			// NOTE: The reason we have to make a clone of the packet here is because multiversion implementations will downgrade the packet
			// and Oomph, instead of using the regular chunk packet sent by the server, will use the one modified by the multiversion implementation
			// since it is a pointer.
			cpk := *pk
			p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
				ack.AckWorldUpdateChunks,
				&cpk,
			))
		} else if p.MovementMode == player.AuthorityModeComplete {
			ack.Instant(ack.AckWorldUpdateChunks, p, pk)
		}
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

func (h *ChunksHandler) validateInteraction(p *player.Player, pk *packet.InventoryTransaction) bool {
	dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}

	// No point in validating an air click...
	if dat.ActionType == protocol.UseItemActionClickAir || dat.ClickedPosition.Len() > 1 {
		h.initalInteractionAccepted = true
		return true
	}

	// Ignore duplicate inputs made by the client to save CPU.
	if h.prevPlaceRequest != nil && dat.BlockRuntimeID == h.prevPlaceRequest.BlockRuntimeID && dat.BlockFace == h.prevPlaceRequest.BlockFace &&
		dat.BlockPosition == h.prevPlaceRequest.BlockPosition && dat.HotBarSlot == h.prevPlaceRequest.HotBarSlot &&
		dat.Position == h.prevPlaceRequest.Position && dat.ClickedPosition == h.prevPlaceRequest.ClickedPosition {
		return false
	}

	// On newer versions of the game (1.21.20+), we are able to determine wether the input was from a
	// simulation frame, or from the player itself. However, on older versions there's no other way to
	// distinguish this besides a zero-vector click position that is usually from jump-bridging.
	var isInitalInput bool
	if p.Conn().Proto().ID() >= player.GameVersion1_21_20 {
		isInitalInput = dat.TriggerType == protocol.TriggerTypePlayerInput
	} else {
		isInitalInput = dat.ClickedPosition.LenSqr() > 0
	}

	if !isInitalInput {
		return h.initalInteractionAccepted
	}

	defer func() {
		h.initalInteraction = dat
	}()

	blockPos := cube.Pos{int(dat.BlockPosition.X()), int(dat.BlockPosition.Y()), int(dat.BlockPosition.Z())}
	interactedBlock := p.World.Block(df_cube.Pos(blockPos))
	interactPos := blockPos.Vec3().Add(dat.ClickedPosition)

	if len(utils.BlockBoxes(interactedBlock, blockPos, p.World)) == 0 {
		h.initalInteractionAccepted = true
		return true
	}

	mDat := p.Handler(HandlerIDMovement).(*MovementHandler)
	if !mDat.s.Reliable(p) {
		return true
	}

	eyePos := mDat.Position
	if mDat.Sneaking {
		eyePos[1] += 1.54
	} else {
		eyePos[1] += 1.62
	}

	// We have 5 blocks here for leniency since we don't account for the interpolated camera position.
	if eyePos.Sub(interactPos).Len() > 5.0 {
		h.initalInteractionAccepted = false
		p.Popup("Interaction denied - too far away.")
		return false
	}

	// Check for all the blocks in between the interaction position and the player's eye position. If any blocks intersect
	// with the line between the player's eye position and the interaction position, the interaction is cancelled.
	for _, intersectingBlockPos := range game.BlocksBetween(eyePos, interactPos) {
		flooredPos := df_cube.Pos{int(intersectingBlockPos[0]), int(intersectingBlockPos[1]), int(intersectingBlockPos[2])}
		if flooredPos == df_cube.Pos(blockPos) {
			continue
		}

		intersectingBlock := p.World.Block(flooredPos)
		iBBs := utils.BlockBoxes(intersectingBlock, cube.Pos(flooredPos), p.World)
		if len(iBBs) == 0 {
			continue
		}

		// Iterate through all the block's bounding boxes to check if it is in the way of the interaction position.
		for _, iBB := range iBBs {
			iBB = iBB.Translate(intersectingBlockPos)

			// If there is an intersection, the interaction is invalid.
			if _, ok := trace.BBoxIntercept(iBB, eyePos, interactPos); ok {
				p.Popup("Interaction denied - block is in the way.")
				h.initalInteractionAccepted = false
				return false
			}
		}
	}

	h.initalInteractionAccepted = true
	return true
}

func (h *ChunksHandler) tryPlaceBlock(p *player.Player, pk *packet.InventoryTransaction) {
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
	fb := p.World.Block(df_cube.Pos(replacePos))

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
	p.World.SetBlock(df_cube.Pos(replacePos), b, nil)
}

// noinspection ALL
//
//go:linkname chunk_subChunkDecode github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func chunk_subChunkDecode(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
