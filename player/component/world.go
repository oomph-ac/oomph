package component

import (
	"fmt"
	"math"
	"sync"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	cloudpacket "github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/utils"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	MaxAllowedPendingBlobs = 4096
)

var (
	legacyAirIDs = make(map[int32]uint32)
	legacyMu     sync.RWMutex
)

// WorldUpdaterComponent is a component that handles block and chunk updates to the world of the member player.
type WorldUpdaterComponent struct {
	mPlayer *player.Player

	chunkRadius       int32
	serverChunkRadius int32

	clientPlacedBlocks  map[df_cube.Pos]*chainedBlockPlacement
	pendingBlockUpdates map[df_cube.Pos]uint32
	batchedBlockUpdates *acknowledgement.UpdateBlockBatch

	breakingBlockPos *protocol.BlockPos
	prevPlaceRequest *protocol.UseItemTransactionData

	initalInteractionAccepted bool

	pendingBlobs map[uint64][]byte
}

func NewWorldUpdaterComponent(p *player.Player) *WorldUpdaterComponent {
	return &WorldUpdaterComponent{
		mPlayer:      p,
		chunkRadius:  1_000_000_000,
		pendingBlobs: make(map[uint64][]byte, MaxAllowedPendingBlobs),

		clientPlacedBlocks:  make(map[df_cube.Pos]*chainedBlockPlacement),
		pendingBlockUpdates: make(map[df_cube.Pos]uint32),
		batchedBlockUpdates: acknowledgement.NewUpdateBlockBatchACK(p),

		initalInteractionAccepted: true,
	}
}

func (c *WorldUpdaterComponent) UseItemData() *protocol.UseItemTransactionData {
	return c.prevPlaceRequest
}

// HandleSubChunk handles a SubChunk packet from the server.
func (c *WorldUpdaterComponent) HandleSubChunk(pk *packet.SubChunk) {
	if !c.mPlayer.Ready {
		c.mPlayer.ACKs().Add(acknowledgement.NewPlayerInitalizedACK(c.mPlayer))
	}

	if pk.CacheEnabled {
		c.mPlayer.Disconnect(game.ErrorChunkCacheUnsupported)
		return
	}

	dimension, ok := df_world.DimensionByID(int(pk.Dimension))
	if !ok {
		dimension = df_world.Overworld
	}

	buf := internal.NewChunkBuf()
	defer internal.PutChunkBuf(buf)
	var bufUsed bool

	newChunks := make(map[protocol.ChunkPos]*chunk.Chunk)
	for _, entry := range pk.SubChunkEntries {
		chunkPos := protocol.ChunkPos{
			pk.Position[0] + int32(entry.Offset[0]),
			pk.Position[2] + int32(entry.Offset[2]),
		}
		var ch *chunk.Chunk
		if new, ok := newChunks[chunkPos]; ok {
			ch = new
			c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "reusing chunk in map %v", chunkPos)
		} else if existing := c.mPlayer.World().GetChunk(chunkPos); existing != nil {
			// We assume that the existing chunk is not cached because the cache does not support SubChunks for the time being.
			ch = existing
			c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "using existing chunk %v", chunkPos)
		} else {
			ch = chunk.New(world.AirRuntimeID, dimension.Range())
			newChunks[chunkPos] = ch
			c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "new chunk at %v", chunkPos)
		}

		switch entry.Result {
		case protocol.SubChunkResultSuccess:
			if bufUsed {
				buf.Reset()
			}
			bufUsed = true
			buf.Write(entry.RawPayload)

			cachedSub, err := world.CacheSubChunk(buf, ch, chunkPos)
			if err != nil {
				c.mPlayer.Disconnect(fmt.Sprintf(game.ErrorInternalDecodeChunk, err))
				continue
			}
			ch.Sub()[cachedSub.Layer()] = cachedSub.SubChunk()
			c.mPlayer.World().AddSubChunk(chunkPos, cachedSub.Hash())
			c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "cached subchunk %d at %v", cachedSub.Layer(), chunkPos)
		case protocol.SubChunkResultSuccessAllAir:
			c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "all-air chunk at %v", chunkPos)
		default:
			c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "no subchunk data for %v (result=%d)", chunkPos, entry.Result)
			continue
		}
	}

	for pos, newChunk := range newChunks {
		c.mPlayer.World().AddChunk(pos, world.ChunkInfo{Chunk: newChunk, Cached: false})
		c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "(sub) added chunk at %v", pos)
	}
}

// HandleLevelChunk handles a LevelChunk packet from the server.
func (c *WorldUpdaterComponent) HandleLevelChunk(pk *packet.LevelChunk) {
	if !c.mPlayer.Ready {
		c.mPlayer.ACKs().Add(acknowledgement.NewPlayerInitalizedACK(c.mPlayer))
	}

	// Check if this LevelChunk packet is compatiable with oomph's handling.
	if pk.SubChunkCount == protocol.SubChunkRequestModeLimited || pk.SubChunkCount == protocol.SubChunkRequestModeLimitless {
		//c.mPlayer.Log().Debug("cannot debug chunk due to subchunk request mode unsupported", "subChunkCount", pk.SubChunkCount)
		return
	}
	//acknowledgement.NewChunkUpdateACK(c.mPlayer, pk).Run()

	// Oomph should be responsible for distributing blobs to the client - not the server.
	if pk.CacheEnabled {
		c.mPlayer.Disconnect(game.ErrorChunkCacheUnsupported)
		return
	}
	cachedChunk, err := world.CacheChunk(pk)
	if err != nil {
		c.mPlayer.Disconnect(fmt.Sprintf(game.ErrorInternalDecodeChunk, err))
		return
	}
	c.mPlayer.World().AddChunk(pk.Position, world.ChunkInfo{
		Cached: true,
		Hash:   cachedChunk.Hash(),
		Chunk:  cachedChunk.Chunk(),
	})
	c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "added chunk at %v", pk.Position)

	// Check first if the client supports the chunk cache - if not, we don't need to do anything.
	// We also check that the chunk being sent isn't an empty air chunk (sent by spectrum? idk what it's coming from)
	if !c.mPlayer.UseChunkCache() || (cachedChunk.Hash().Hi == 15870946467309531877 && cachedChunk.Hash().Lo == 14339477491833271119) {
		//fmt.Printf("not caching chunk at %v\n", pk.Position)
		return
	}

	chunkBlobs := cachedChunk.Blobs()
	chunkFooter := cachedChunk.Footer()

	if !c.mPlayer.IsVersion(protocol.CurrentProtocol) {
		legacyBlobs, legacyFooter, ok := cachedChunk.LegacyData(c.mPlayer.Version)
		if !ok {
			legacyBlobs, legacyFooter, ok = c.updateAndSetLegacyData(pk, cachedChunk)
			if !ok {
				return
			}
		}
		chunkBlobs = legacyBlobs
		chunkFooter = legacyFooter
	}

	c.mPlayer.WithPacketCtx(func(ctx *context.HandlePacketContext) {
		ctx.Cancel()
	})
	newChunkPk := &CustomLevelChunk{
		Position:      pk.Position,
		Dimension:     pk.Dimension,
		SubChunkCount: uint32(len(chunkBlobs)) - 1,
		CacheEnabled:  true,
		RawPayload:    chunkFooter,
		BlobHashes:    make([]uint64, 0, len(chunkBlobs)),
	}
	for _, blob := range chunkBlobs {
		newChunkPk.BlobHashes = append(newChunkPk.BlobHashes, blob.Hash)
		if !c.addPendingBlob(blob.Hash, blob.Payload) {
			c.mPlayer.Disconnect(game.ErrorTooManyChunkBlobsPending)
			return
		}
	}
	c.mPlayer.SendPacketToClient(newChunkPk)
}

// HandleUpdateBlock handles an UpdateBlock packet from the server.
func (c *WorldUpdaterComponent) HandleUpdateBlock(pk *packet.UpdateBlock) {
	pos := df_cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
	if pk.Layer != 0 {
		c.mPlayer.Log().Debug("unsupported layer update block", "layer", pk.Layer, "block", pk.NewBlockRuntimeID, "pos", pos)
		return
	}
	c.AddPendingUpdate(pos, pk.NewBlockRuntimeID)
}

// HandleUpdateSubChunkBlocks handles an UpdateSubChunkBlocks packet from the server.
func (c *WorldUpdaterComponent) HandleUpdateSubChunkBlocks(pk *packet.UpdateSubChunkBlocks) {
	if !c.mPlayer.Ready {
		c.mPlayer.ACKs().Add(acknowledgement.NewPlayerInitalizedACK(c.mPlayer))
	}
	for _, entry := range pk.Blocks {
		c.AddPendingUpdate(df_cube.Pos{int(entry.BlockPos.X()), int(entry.BlockPos.Y()), int(entry.BlockPos.Z())}, entry.BlockRuntimeID)
	}
	for _, entry := range pk.Extra {
		c.AddPendingUpdate(df_cube.Pos{int(entry.BlockPos.X()), int(entry.BlockPos.Y()), int(entry.BlockPos.Z())}, entry.BlockRuntimeID)
	}
}

func (c *WorldUpdaterComponent) HandleClientBlobStatus(pk *packet.ClientCacheBlobStatus) {
	for _, blob := range pk.HitHashes {
		//fmt.Printf("hit blob: %d\n", blob)
		c.removePendingBlob(blob)
	}

	resp := &CustomClientCacheMissResponse{Blobs: make([]protocol.CacheBlob, 0, len(pk.MissHashes))}
	for _, blobHash := range pk.MissHashes {
		//fmt.Printf("missed blob: %d\n", blobHash)
		blob, ok := c.pendingBlobs[blobHash]
		if !ok {
			//c.mPlayer.Log().Debug("unable to find pending blob", "hash", blobHash)
			continue
		}
		resp.Blobs = append(resp.Blobs, protocol.CacheBlob{
			Hash:    blobHash,
			Payload: blob,
		})
		//fmt.Printf("sent blob: %d\n", blobHash)
	}
	if len(resp.Blobs) > 0 {
		c.mPlayer.SendPacketToClient(resp)
	}
}

// AttemptItemInteractionWithBlock attempts a block placement request from the client. It returns false if the simulation is unable
// to place a block at the given position.
func (c *WorldUpdaterComponent) AttemptItemInteractionWithBlock(pk *packet.InventoryTransaction) bool {
	dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}

	snapshot := &cloudpacket.BlockInteractionSnapshot{CloudID: c.mPlayer.CloudID()}
	if c.prevPlaceRequest == nil {
		snapshot.SetActionType(dat.ActionType)
		snapshot.SetTriggerType(dat.TriggerType)
		snapshot.SetClientPrediction(dat.ClientPrediction)
		snapshot.SetBlockFace(dat.BlockFace)
		snapshot.SetBlockPos(dat.BlockPosition)
		snapshot.SetReportedPos(dat.Position)
		snapshot.SetClickedPos(dat.ClickedPosition)
	} else {
		if dat.ActionType != c.prevPlaceRequest.ActionType {
			snapshot.SetActionType(dat.ActionType)
		}
		if dat.TriggerType != c.prevPlaceRequest.TriggerType {
			snapshot.SetTriggerType(dat.TriggerType)
		}
		if dat.ClientPrediction != c.prevPlaceRequest.ClientPrediction {
			snapshot.SetClientPrediction(dat.ClientPrediction)
		}
		if dat.BlockFace != c.prevPlaceRequest.BlockFace {
			snapshot.SetBlockFace(dat.BlockFace)
		}
		if dat.BlockPosition != c.prevPlaceRequest.BlockPosition {
			snapshot.SetBlockPos(dat.BlockPosition)
		}
		if dat.Position != c.prevPlaceRequest.Position {
			snapshot.SetReportedPos(dat.Position)
		}
		if dat.ClickedPosition != c.prevPlaceRequest.ClickedPosition {
			snapshot.SetClickedPos(dat.ClickedPosition)
		}
	}
	c.mPlayer.WriteToCloud(snapshot)
	c.prevPlaceRequest = dat

	if dat.ActionType != protocol.UseItemActionClickBlock {
		return true
	}

	holding := c.mPlayer.Inventory().Holding()
	_, heldIsBlock := holding.Item().(df_world.Block)
	if heldIsBlock && c.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) && dat.ClientPrediction == protocol.ClientPredictionFailure {
		// We don't want to force a sync here, as the client has already predicted their interaction has failed.
		return false
	}

	// Ignore if the block face the client sends is not valid.
	if dat.BlockFace < 0 || dat.BlockFace > 5 {
		return false
	}

	clickedBlockPos := utils.BlockToCubePos(dat.BlockPosition)
	dfClickedBlockPos := df_cube.Pos(clickedBlockPos)
	replacingBlock := c.mPlayer.World().Block(dfClickedBlockPos)

	// It is impossible for the replacing block to be air, as the client would send UseItemActionClickAir instead of UseItemActionClickBlock.
	_, isAir := replacingBlock.(block.Air)
	if isAir {
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "interaction denied: clicked block at %v is air", dfClickedBlockPos)
		c.mPlayer.SyncBlock(dfClickedBlockPos)
		c.mPlayer.SyncBlock(dfClickedBlockPos.Side(df_cube.Face(dat.BlockFace)))
		c.mPlayer.Inventory().ForceSync()
		return false
	} else if placement, hasPlacement := c.clientPlacedBlocks[dfClickedBlockPos]; hasPlacement && c.mPlayer.Opts().Network.MaxGhostBlockChain >= 0 {
		chainSize, anyConfirmed := placement.ghostBlockChain()
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "gbChainSize=%d anyConfirmed=%t", chainSize, anyConfirmed)
		if anyConfirmed && !placement.placementAllowed && chainSize+1 > c.mPlayer.Opts().Network.MaxGhostBlockChain {
			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "interaction denied: clicked block is in ghost block chain that exceeds limit")
			c.mPlayer.Popup("<red>Ghost block(s) cancelled</red>")
			c.mPlayer.SyncBlock(dfClickedBlockPos)
			c.mPlayer.SyncBlock(dfClickedBlockPos.Side(df_cube.Face(dat.BlockFace)))
			c.mPlayer.Inventory().ForceSync()
			return false
		}
	}

	// Check if the clicked block is too far away from the player's position.
	prevPos, currPos := c.mPlayer.Movement().LastPos(), c.mPlayer.Movement().Pos()
	if c.mPlayer.Movement().Sneaking() {
		prevPos[1] += game.SneakingPlayerHeightOffset
		currPos[1] += game.SneakingPlayerHeightOffset
	} else {
		prevPos[1] += game.DefaultPlayerHeightOffset
		currPos[1] += game.DefaultPlayerHeightOffset
	}

	closestDistance := float32(math.MaxFloat32 - 1)
	blockBBoxes := utils.BlockBoxes(replacingBlock, clickedBlockPos, c.mPlayer.World())
	if len(blockBBoxes) == 0 {
		blockBBoxes = []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	}
	for _, bb := range blockBBoxes {
		bb = bb.Translate(clickedBlockPos.Vec3())
		closestOrigin := game.ClosestPointInLineToPoint(prevPos, currPos, game.BBoxCenter(bb))
		if dist := game.ClosestPointToBBox(closestOrigin, bb).Sub(closestOrigin).Len(); dist < closestDistance {
			closestDistance = dist
		}
	}

	// TODO: Figure out why it seems that this works for both creative and survival mode. Though, we will exempt creative mode from this check for now...
	if c.mPlayer.GameMode != packet.GameTypeCreative && closestDistance > game.MaxBlockInteractionDistance {
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "interaction too far away (%.4f blocks)", closestDistance)
		c.mPlayer.Popup("<red>Interaction too far away (%.2f blocks)</red>", closestDistance)
		c.mPlayer.SyncBlock(dfClickedBlockPos)
		c.mPlayer.SyncBlock(dfClickedBlockPos.Side(df_cube.Face(dat.BlockFace)))
		c.mPlayer.Inventory().ForceSync()
		return false
	}

	if act, ok := replacingBlock.(block.Activatable); ok && (!c.mPlayer.Movement().PressingSneak() || holding.Empty()) {
		utils.ActivateBlock(c.mPlayer, act, df_cube.Pos(clickedBlockPos), c.mPlayer.World())
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "called utils.ActivateBlock: clicked block is activatable")
		return true
	}

	heldItem := holding.Item()
	c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "item in hand (slot %d): %T", c.mPlayer.Inventory().HeldSlot(), heldItem)
	switch heldItem := heldItem.(type) {
	case *block.Air:
		// This only happens when Dragonfly is unsure of what the item is (unregistered), so we use the client-authoritative block in hand.
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "called c.mPlayer.PlaceBlock: using client-authoritative block in hand")
		if b, ok := df_world.BlockByRuntimeID(uint32(dat.HeldItem.Stack.BlockRuntimeID)); ok {
			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "placing block with runtime ID: %d", dat.HeldItem.Stack.BlockRuntimeID)

			// If the block at the position is not replacable, we want to place the block on the side of the block.
			replaceBlockPos := clickedBlockPos
			if replaceable, ok := replacingBlock.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
				replaceBlockPos = clickedBlockPos.Side(cube.Face(dat.BlockFace))
			}
			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "using client-authoritative block in hand: %T", b)
			c.mPlayer.PlaceBlock(df_cube.Pos(clickedBlockPos), df_cube.Pos(replaceBlockPos), df_cube.Face(dat.BlockFace), b)
		} else {
			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "unable to find block with runtime ID: %d", dat.HeldItem.Stack.BlockRuntimeID)
		}
	case nil:
		// The player has nothing in this slot, ignore the block placement.
		// FIXME: It seems some blocks aren't implemented by Dragonfly and will therefore seem to be air when
		// it is actually a valid block.
		//c.mPlayer.Popup("<red>No item in hand.</red>")
		//c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "Block placement denied: no item in hand.")
		return true
	case item.UsableOnBlock:
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "running interaction w/ item.UsableOnBlock")
		utils.UseOnBlock(utils.UseOnBlockOpts{
			Placer:          c.mPlayer,
			UseableOnBlock:  heldItem,
			ClickedBlockPos: df_cube.Pos(clickedBlockPos),
			ReplaceBlockPos: df_cube.Pos(clickedBlockPos),
			ClickPos:        game.Vec32To64(dat.ClickedPosition),
			Face:            df_cube.Face(dat.BlockFace),
			Src:             c.mPlayer.World(),
		})
	case df_world.Block:
		if _, isGlowstone := heldItem.(block.Glowstone); isGlowstone {
			if utils.BlockName(c.mPlayer.World().Block(df_cube.Pos(clickedBlockPos))) == "minecraft:respawn_anchor" {
				c.mPlayer.Dbg.Notify(player.DebugModeBlockInteraction, true, "charging respawn anchor with glowstone")
				return true
			}
		}

		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "placing world.Block")

		// If the block at the position is not replacable, we want to place the block on the side of the block.
		replaceBlockPos := clickedBlockPos
		if replaceable, ok := replacingBlock.(block.Replaceable); !ok || !replaceable.ReplaceableBy(heldItem) {
			replaceBlockPos = clickedBlockPos.Side(cube.Face(dat.BlockFace))
		}
		c.mPlayer.PlaceBlock(df_cube.Pos(clickedBlockPos), df_cube.Pos(replaceBlockPos), df_cube.Face(dat.BlockFace), heldItem)
	default:
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "unsupported item type for block placement: %T", heldItem)
	}
	return true
}

func (c *WorldUpdaterComponent) ValidateInteraction(pk *packet.InventoryTransaction) bool {
	if gm := c.mPlayer.GameMode; gm != packet.GameTypeSurvival && gm != packet.GameTypeAdventure {
		return true
	}

	dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}
	if dat.ActionType != protocol.UseItemActionClickBlock {
		c.initalInteractionAccepted = true
		return true
	}
	if c.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) && dat.ClientPrediction != protocol.ClientPredictionSuccess {
		return true
	}

	blockPos := cube.Pos{int(dat.BlockPosition.X()), int(dat.BlockPosition.Y()), int(dat.BlockPosition.Z())}
	interactPos := blockPos.Vec3().Add(dat.ClickedPosition)
	interactedBlock := c.mPlayer.World().Block(df_cube.Pos(blockPos))

	if _, isActivatable := interactedBlock.(block.Activatable); !isActivatable {
		return true
	}
	if len(utils.BlockBoxes(interactedBlock, blockPos, c.mPlayer.World())) == 0 {
		c.initalInteractionAccepted = true
		return true
	}

	prevPos, currPos := c.mPlayer.Movement().LastPos(), c.mPlayer.Movement().Pos()
	if c.mPlayer.Movement().Sneaking() {
		prevPos[1] += game.SneakingPlayerHeightOffset
		currPos[1] += game.SneakingPlayerHeightOffset
	} else {
		prevPos[1] += game.DefaultPlayerHeightOffset
		currPos[1] += game.DefaultPlayerHeightOffset
	}
	closestEyePos := game.ClosestPointInLineToPoint(prevPos, currPos, interactPos)

	if dist := closestEyePos.Sub(interactPos).Len(); dist > game.MaxBlockInteractionDistance {
		c.initalInteractionAccepted = false
		c.mPlayer.Dbg.Notify(player.DebugModeBlockInteraction, true, "Interaction denied: too far away (%.4f blocks).", dist)
		//c.mPlayer.NMessage("<red>Interaction denied: too far away.</red>")
		return false
	}

	// Check for all the blocks in between the interaction position and the player's eye position. If any blocks intersect
	// with the line between the player's eye position and the interaction position, the interaction is cancelled.
	checkedPositions := make(map[df_cube.Pos]struct{})
	for intersectingBlockPos := range game.BlocksBetween(closestEyePos, interactPos, 49) {
		flooredPos := df_cube.Pos{int(intersectingBlockPos[0]), int(intersectingBlockPos[1]), int(intersectingBlockPos[2])}
		if flooredPos == df_cube.Pos(blockPos) {
			continue
		}

		// Make sure we don't iterate through the same block multiple times.
		if _, ok := checkedPositions[flooredPos]; ok {
			continue
		}
		checkedPositions[flooredPos] = struct{}{}

		intersectingBlock := c.mPlayer.World().Block(flooredPos)

		switch intersectingBlock.(type) {
		case block.InvisibleBedrock, block.Barrier:
			continue
		}

		iBBs := utils.BlockBoxes(intersectingBlock, cube.Pos(flooredPos), c.mPlayer.World())
		if len(iBBs) == 0 {
			continue
		}

		// Iterate through all the block's bounding boxes to check if it is in the way of the interaction position.
		for _, iBB := range iBBs {
			iBB = iBB.Translate(intersectingBlockPos)

			// If there is an intersection, the interaction is invalid.
			if _, ok := trace.BBoxIntercept(iBB, closestEyePos, interactPos); ok {
				//c.mPlayer.NMessage("<red>Interaction denied: block obstructs path.</red>")
				c.mPlayer.Dbg.Notify(player.DebugModeBlockInteraction, true, "Interaction denied: block obstructs path.")
				c.initalInteractionAccepted = false
				return false
			}
		}
	}

	c.initalInteractionAccepted = true
	return true
}

// SetServerChunkRadius sets the server chunk radius of the world updater component.
func (c *WorldUpdaterComponent) SetServerChunkRadius(radius int32) {
	c.serverChunkRadius = radius
	c.chunkRadius = radius
}

// SetChunkRadius sets the chunk radius of the world updater component.
func (c *WorldUpdaterComponent) SetChunkRadius(radius int32) {
	if radius > c.serverChunkRadius && c.serverChunkRadius != 0 {
		radius = c.serverChunkRadius
	}
	c.chunkRadius = radius
}

// ChunkRadius returns the chunk radius of the world udpater component.
func (c *WorldUpdaterComponent) ChunkRadius() int32 {
	return c.chunkRadius
}

// SetBlockBreakPos sets the block breaking pos of the world updater component.
func (c *WorldUpdaterComponent) SetBlockBreakPos(pos *protocol.BlockPos) {
	c.breakingBlockPos = pos
}

// BlockBreakPos returns the block breaking pos of the world updater component.
func (c *WorldUpdaterComponent) BlockBreakPos() *protocol.BlockPos {
	return c.breakingBlockPos
}

func (c *WorldUpdaterComponent) QueueBlockPlacement(clickedBlockPos, placedBlockPos df_cube.Pos, parentFace df_cube.Face) {
	// Any number of ghost blocks allowed when MaxGhostBlockChain is less than zero - so we should skip handling to save resources.
	if c.mPlayer.Opts().Network.MaxGhostBlockChain < 0 {
		return
	}

	c.clientPlacedBlocks[placedBlockPos] = newChainedBlockPlacement(
		df_world.BlockRuntimeID(c.mPlayer.World().Block(placedBlockPos)),
		parentFace,
		c.clientPlacedBlocks[clickedBlockPos],
	)
}

func (c *WorldUpdaterComponent) AddPendingUpdate(pos df_cube.Pos, blockRuntimeID uint32) {
	c.batchedBlockUpdates.SetBlock(pos, blockRuntimeID)
	c.pendingBlockUpdates[pos] = blockRuntimeID
}

func (c *WorldUpdaterComponent) HasPendingUpdate(pos df_cube.Pos) bool {
	_, ok := c.pendingBlockUpdates[pos]
	return ok
}

func (c *WorldUpdaterComponent) RemovePendingUpdate(pos df_cube.Pos, blockRuntimeID uint32) {
	if pendingBlockRuntimeID, ok := c.pendingBlockUpdates[pos]; ok && pendingBlockRuntimeID == blockRuntimeID {
		delete(c.pendingBlockUpdates, pos)
	}
}

func (c *WorldUpdaterComponent) Flush() {
	if !c.batchedBlockUpdates.HasUpdates() {
		return
	}

	networkOpts := c.mPlayer.Opts().Network
	blockAckTimeout := int64(networkOpts.MaxBlockUpdateDelay)
	if blockAckTimeout < 0 {
		blockAckTimeout = 1_000_000_000
	}
	cutoff := int64(networkOpts.GlobalMovementCutoffThreshold)
	noLagComp := cutoff >= 0 && c.mPlayer.ServerTick-c.mPlayer.ClientTick >= cutoff
	maxChain := networkOpts.MaxGhostBlockChain

	blockUpdates := c.batchedBlockUpdates.Blocks()
	for pos, bRuntimeID := range blockUpdates {
		b, ok := df_world.BlockByRuntimeID(bRuntimeID)
		if !ok {
			c.mPlayer.Log().Warn("unable to find block with runtime ID", "blockRuntimeID", bRuntimeID)
			b = block.Air{}
		}
		// We will consider the block placement rejected if the new block runtime ID is equal to the previous block runtime ID pre-placement.
		if pl, ok := c.clientPlacedBlocks[pos]; ok && networkOpts.MaxGhostBlockChain >= 0 {
			placementAllowed := df_world.BlockRuntimeID(b) != pl.prePlacedBlockRID
			if !placementAllowed {
				c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "placement NOT allowed at %v", pos)
			} else {
				c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "placement allowed at %v", pos)
			}

			// If the value is at least 1, the user wants to allow a certain amount of ghost blocks.
			if maxChain >= 1 {
				pl.setPlacementAllowed(placementAllowed)
			} else if maxChain == 0 && !placementAllowed {
				// The user wants to refuse compensation for ghost blocks, so we will set the block in the world immediately.
				c.mPlayer.Popup("<red>Ghost blocks not allowed.</red>")
				c.batchedBlockUpdates.RemoveBlock(pos)
				c.mPlayer.World().SetBlock(pos, b, nil)
				c.RemovePendingUpdate(pos, bRuntimeID)
			}
		}
	}

	c.batchedBlockUpdates.SetExpiry(blockAckTimeout)
	if noLagComp {
		c.batchedBlockUpdates.Run()
	} else {
		c.mPlayer.ACKs().Add(c.batchedBlockUpdates)
	}
	c.batchedBlockUpdates = acknowledgement.NewUpdateBlockBatchACK(c.mPlayer)
}

func (c *WorldUpdaterComponent) Tick() {
	for pos, pl := range c.clientPlacedBlocks {
		pl.remainingTicks--
		if pl.remainingTicks <= 0 {
			pl.destroy()
			delete(c.clientPlacedBlocks, pos)
		}
	}
}

const (
	// maxChainedBlockPlacementLifetime is how long in ticks a block placement will be stored in the world updater component.
	// We need this because we don't want to remove the block placement immediately when the server notifies us (or also lack of)
	// of whether the block placement was allowed or not. Some server softwares send multiple UpdateBlock packets for the same positions (PMMP)
	// and the proxy may not recieve all of them at the same time. Ping me on Discord for more specific information if it doesn't make sense,
	// because I am rushing this note.
	maxChainedBlockPlacementLifetime = 10 * player.TicksPerSecond
	maxGhostBlockChainSize           = 100
)

// chainedBlockPlacement is a structure that is able to represent chained pending block placements. This is intended to be used
// to prevent placements against ghost blocks if they have been confirmed by the server. However - we also do not want to mess up with
// the movement of players who are walking on ghost blocks and still have those lag compensated. So, we use this for checking
// if a block that was placed against was in a chain of ghost blocks, and deny the placement if so.
type chainedBlockPlacement struct {
	placementAllowed   bool
	placementConfirmed bool
	remainingTicks     uint16
	prePlacedBlockRID  uint32
	parentFace         df_cube.Face
	connections        [6]*chainedBlockPlacement
}

func newChainedBlockPlacement(
	prePlacedBlockRID uint32,
	face df_cube.Face,
	parent *chainedBlockPlacement,
) *chainedBlockPlacement {
	pl := &chainedBlockPlacement{
		prePlacedBlockRID: prePlacedBlockRID,
		parentFace:        -1,
		remainingTicks:    maxChainedBlockPlacementLifetime,
		placementAllowed:  true,
	}
	if parent != nil {
		pl.parentFace = face.Opposite()
		pl.connections[pl.parentFace] = parent
		parent.connections[face] = pl
	}
	return pl
}

func (pl *chainedBlockPlacement) ghostBlockChain() (int, bool) {
	if pl.placementAllowed {
		return 0, true
	} else if pl.parentFace == -1 {
		return 1, pl.placementConfirmed
	}
	size := 1
	anyConfirmed := pl.placementConfirmed
	parent := pl.connections[pl.parentFace]
	for parent != nil {
		// If the placement was allowed, it is not part of the ghost block chain.
		if parent.placementAllowed {
			anyConfirmed = true
			break
		}
		size++
		anyConfirmed = anyConfirmed || parent.placementConfirmed
		if parent.parentFace == -1 || size == maxGhostBlockChainSize {
			break
		}
		parent = parent.connections[parent.parentFace]
	}
	return size, anyConfirmed
}

func (pl *chainedBlockPlacement) setPlacementAllowed(allowed bool) {
	// Usually happens when servers try to send block updates too quickly, resulting in block flickering.
	if pl.placementConfirmed && pl.placementAllowed {
		return
	}

	// Iterative DFS to avoid deep recursion on long chains
	type nodeStep struct {
		node *chainedBlockPlacement
		step byte
	}

	visited := make(map[*chainedBlockPlacement]struct{})
	stack := make([]nodeStep, 0, 8)
	stack = append(stack, nodeStep{node: pl, step: 0})

	for len(stack) > 0 {
		ns := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		n := ns.node
		step := ns.step

		if _, ok := visited[n]; ok {
			continue
		}
		visited[n] = struct{}{}

		// Preserve original early-exit semantics per node
		if (n.placementConfirmed && n.placementAllowed) || step >= maxGhostBlockChainSize {
			continue
		}

		n.placementAllowed = allowed
		n.placementConfirmed = true

		for _, face := range df_cube.Faces() {
			if face == n.parentFace {
				continue
			}
			child := n.connections[face]
			if child != nil {
				stack = append(stack, nodeStep{node: child, step: step + 1})
			}
		}
	}
}

func (pl *chainedBlockPlacement) destroy() {
	for _, face := range df_cube.Faces() {
		child := pl.connections[face]
		if child != nil {
			child.connections[face.Opposite()] = nil
			child.connections[face] = nil
			child.parentFace = -1
		}
	}
}

func (c *WorldUpdaterComponent) addPendingBlob(hash uint64, data []byte) bool {
	if len(c.pendingBlobs) >= MaxAllowedPendingBlobs {
		return false
	}
	if _, ok := c.pendingBlobs[hash]; !ok {
		c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "adding pending blob: %d", hash)
		c.pendingBlobs[hash] = data
	}
	return true
}

func (c *WorldUpdaterComponent) removePendingBlob(hash uint64) {
	// TODO: Implement multi-version support for blobs.
	if _, ok := c.pendingBlobs[hash]; ok {
		c.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "removing pending blob: %d", hash)
		delete(c.pendingBlobs, hash)
	}
}

func (c *WorldUpdaterComponent) updateAndSetLegacyData(chunkPk *packet.LevelChunk, cachedChunk *world.CachedChunk) ([]protocol.CacheBlob, []byte, bool) {
	legacyMu.RLock()
	legacyAirRID, ok := legacyAirIDs[c.mPlayer.Version]
	legacyMu.RUnlock()

	if !ok {
		blockPk := &packet.UpdateBlock{NewBlockRuntimeID: world.AirRuntimeID}
		downgraded := c.mPlayer.Conn().Proto().ConvertFromLatest(blockPk, c.mPlayer.Conn())
		if len(downgraded) != 1 {
			c.mPlayer.Log().Debug("unable to set legacy - too many packets returned from MV", "expected", 1, "got", len(downgraded))
			return nil, nil, false
		}
		updateBlock, ok := downgraded[0].(*packet.UpdateBlock)
		if !ok {
			c.mPlayer.Log().Debug("unable to set legacy - packet is not a UpdateBlock", "packet", downgraded[0])
			return nil, nil, false
		}
		legacyAirRID = updateBlock.NewBlockRuntimeID
		legacyMu.Lock()
		legacyAirIDs[c.mPlayer.Version] = legacyAirRID
		legacyMu.Unlock()
	}

	downgraded := c.mPlayer.Conn().Proto().ConvertFromLatest(chunkPk, c.mPlayer.Conn())
	if len(downgraded) != 1 {
		c.mPlayer.Log().Debug("unable to set legacy - too many packets returned from MV", "expected", 1, "got", len(downgraded))
		return nil, nil, false
	}
	legacyChunkPk, ok := downgraded[0].(*packet.LevelChunk)
	if !ok {
		c.mPlayer.Log().Debug("unable to set legacy - packet is not a LevelChunk", "packet", downgraded[0])
		return nil, nil, false
	}

	dimension, ok := df_world.DimensionByID(int(legacyChunkPk.Dimension))
	if !ok {
		dimension = df_world.Overworld
	}
	blobs, footer, err := world.FetchChunkFooterAndBlobs(
		legacyChunkPk.RawPayload,
		legacyAirRID,
		int(legacyChunkPk.SubChunkCount),
		dimension,
	)
	if err != nil {
		c.mPlayer.Log().Debug("unable to set legacy - error fetching chunk footer and blobs", "error", err)
		return nil, nil, false
	}
	cachedChunk.SetLegacyData(c.mPlayer.Version, blobs, footer)
	return blobs, footer, true
}

type CustomLevelChunk struct {
	Position        protocol.ChunkPos
	Dimension       int32
	HighestSubChunk uint16
	SubChunkCount   uint32
	CacheEnabled    bool
	BlobHashes      []uint64
	RawPayload      []byte
}

func (pk *CustomLevelChunk) ID() uint32 {
	return packet.IDLevelChunk
}

func (pk *CustomLevelChunk) Marshal(io protocol.IO) {
	io.ChunkPos(&pk.Position)
	io.Varint32(&pk.Dimension)
	io.Varuint32(&pk.SubChunkCount)
	if pk.SubChunkCount == protocol.SubChunkRequestModeLimited {
		io.Uint16(&pk.HighestSubChunk)
	}
	io.Bool(&pk.CacheEnabled)
	if pk.CacheEnabled {
		protocol.FuncSlice(io, &pk.BlobHashes, io.Uint64)
	}
	io.ByteSlice(&pk.RawPayload)
}

// CustomClientCacheMissResponse is a wrapper for the ClientCacheMissResponse packet with the sole intent of being able to
// bypass downgrading from multi-version libraries.
type CustomClientCacheMissResponse struct {
	// Blobs is a list of all blobs that the client sent misses for in the ClientCacheBlobStatus. These blobs
	// hold the data of the blobs with the hashes they are matched with.
	Blobs []protocol.CacheBlob
}

func (pk *CustomClientCacheMissResponse) ID() uint32 {
	return packet.IDClientCacheMissResponse
}

func (pk *CustomClientCacheMissResponse) Marshal(io protocol.IO) {
	protocol.Slice(io, &pk.Blobs)
}
