package component

import (
	"math"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// WorldUpdaterComponent is a component that handles block and chunk updates to the world of the member player.
type WorldUpdaterComponent struct {
	mPlayer *player.Player

	breakingBlockPos          *protocol.BlockPos
	prevPlaceRequest          *protocol.UseItemTransactionData
	chunkRadius               int32
	initalInteractionAccepted bool
}

func NewWorldUpdaterComponent(p *player.Player) *WorldUpdaterComponent {
	return &WorldUpdaterComponent{
		mPlayer:     p,
		chunkRadius: 1_000_000_000,
	}
}

// HandleSubChunk handles a SubChunk packet from the server.
func (c *WorldUpdaterComponent) HandleSubChunk(pk *packet.SubChunk) {
	if !c.mPlayer.Ready {
		c.mPlayer.ACKs().Add(acknowledgement.NewPlayerInitalizedACK(c.mPlayer))
	}
	acknowledgement.NewSubChunkUpdateACK(c.mPlayer, pk).Run()
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
	acknowledgement.NewChunkUpdateACK(c.mPlayer, pk).Run()
}

// HandleUpdateBlock handles an UpdateBlock packet from the server.
func (c *WorldUpdaterComponent) HandleUpdateBlock(pk *packet.UpdateBlock) {
	pos := df_cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
	b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
	if !ok {
		c.mPlayer.Log().Warn("unable to find block with runtime ID", "blockRuntimeID", pk.NewBlockRuntimeID)
		b = block.Air{}
	}

	if pk.Layer != 0 {
		c.mPlayer.Log().Debug("unsupported layer update block", "layer", pk.Layer, "block", utils.BlockName(b), "pos", pos)
		return
	}

	// TODO: Add a block policy to allow servers to determine whether block updates should be lag-compensated or if movement should
	// use the latest world state instantly.
	c.mPlayer.ACKs().Add(acknowledgement.NewUpdateBlockACK(c.mPlayer, pos, b))
}

// HandleUpdateSubChunkBlocks handles an UpdateSubChunkBlocks packet from the server.
func (c *WorldUpdaterComponent) HandleUpdateSubChunkBlocks(pk *packet.UpdateSubChunkBlocks) {
	if !c.mPlayer.Ready {
		c.mPlayer.ACKs().Add(acknowledgement.NewPlayerInitalizedACK(c.mPlayer))
	}

	// TODO: Does the sub-chunk position sent in this packet even matter?
	for _, entry := range pk.Blocks {
		pos := df_cube.Pos{int(entry.BlockPos.X()), int(entry.BlockPos.Y()), int(entry.BlockPos.Z())}
		b, ok := df_world.BlockByRuntimeID(entry.BlockRuntimeID)
		if !ok {
			c.mPlayer.Log().Warn("unable to find block with runtime ID", "blockRuntimeID", entry.BlockRuntimeID)
			b = block.Air{}
		}
		c.mPlayer.ACKs().Add(acknowledgement.NewUpdateBlockACK(c.mPlayer, pos, b))
	}
	for _, entry := range pk.Extra {
		pos := df_cube.Pos{int(entry.BlockPos.X()), int(entry.BlockPos.Y()), int(entry.BlockPos.Z())}
		b, ok := df_world.BlockByRuntimeID(entry.BlockRuntimeID)
		if !ok {
			c.mPlayer.Log().Warn("unable to find block with runtime ID", "blockRuntimeID", entry.BlockRuntimeID)
			b = block.Air{}
		}
		c.mPlayer.ACKs().Add(acknowledgement.NewUpdateBlockACK(c.mPlayer, pos, b))
	}
}

// AttemptItemInteractionWithBlock attempts a block placement request from the client. It returns false if the simulation is unable
// to place a block at the given position.
func (c *WorldUpdaterComponent) AttemptItemInteractionWithBlock(pk *packet.InventoryTransaction) bool {
	dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}

	c.prevPlaceRequest = dat
	if dat.ActionType != protocol.UseItemActionClickBlock {
		return true
	}

	holding := c.mPlayer.Inventory().Holding()
	_, heldIsBlock := holding.Item().(df_world.Block)
	if heldIsBlock && c.mPlayer.VersionInRange(player.GameVersion1_21_20, 99999999) && dat.ClientPrediction == protocol.ClientPredictionFailure {
		// We don't want to force a sync here, as the client has already predicted their interaction has failed.
		return false
	}

	replacePos := utils.BlockToCubePos(dat.BlockPosition)
	dfReplacePos := df_cube.Pos(replacePos)
	replacingBlock := c.mPlayer.World().Block(dfReplacePos)

	// It is impossible for the replacing block to be air, as the client would send UseItemActionClickAir instead of UseItemActionClickBlock.
	if _, isAir := replacingBlock.(block.Air); isAir {
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "interaction denied: clicked block is air on UseItemClickBlock")
		c.mPlayer.SyncBlock(dfReplacePos)
		c.mPlayer.SyncBlock(dfReplacePos.Side(df_cube.Face(dat.BlockFace)))
		c.mPlayer.Inventory().ForceSync()
		return false
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
	blockBBoxes := utils.BlockBoxes(replacingBlock, replacePos, c.mPlayer.World())
	if len(blockBBoxes) == 0 {
		blockBBoxes = []cube.BBox{{}}
	}
	for _, bb := range blockBBoxes {
		bb = bb.Translate(replacePos.Vec3())
		closestOrigin := game.ClosestPointInLineToPoint(prevPos, currPos, game.BBoxCenter(bb))
		if dist := game.ClosestPointToBBox(closestOrigin, bb).Sub(closestOrigin).Len(); dist < closestDistance {
			closestDistance = dist
		}
	}

	// TODO: Figure out why it seems that this works for both creative and survival mode. Though, we will exempt creative mode from this check for now...
	if c.mPlayer.GameMode != packet.GameTypeCreative && closestDistance > game.MaxBlockInteractionDistance {
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "interaction too far away (%.4f blocks)", closestDistance)
		c.mPlayer.Popup("<red>Interaction too far away (%.2f blocks)</red>", closestDistance)
		c.mPlayer.SyncBlock(dfReplacePos)
		c.mPlayer.SyncBlock(dfReplacePos.Side(df_cube.Face(dat.BlockFace)))
		c.mPlayer.Inventory().ForceSync()
		return false
	}

	if act, ok := replacingBlock.(block.Activatable); ok && (!c.mPlayer.Movement().PressingSneak() || holding.Empty()) {
		utils.ActivateBlock(c.mPlayer, act, df_cube.Face(dat.BlockFace), df_cube.Pos(replacePos), game.Vec32To64(dat.ClickedPosition), c.mPlayer.World())
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
			if replaceable, ok := replacingBlock.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
				replacePos = replacePos.Side(cube.Face(dat.BlockFace))
			}

			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "using client-authoritative block in hand: %T", b)
			c.mPlayer.PlaceBlock(df_cube.Pos(replacePos), b, nil)
		} else {
			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "unable to find block with runtime ID: %d", dat.HeldItem.Stack.BlockRuntimeID)
		}
	case nil:
		// The player has nothing in this slot, ignore the block placement.
		// FIXME: It seems some blocks aren't implemented by Dragonfly and will therefore seem to be air when
		// it is actually a valid block.
		//c.mPlayer.NMessage("<red>Block placement denied: no item in hand.</red>")
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "Block placement denied: no item in hand.")
		return true
	case item.UsableOnBlock:
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "running interaction w/ item.UsableOnBlock")
		utils.UseOnBlock(c.mPlayer, heldItem, df_cube.Face(dat.BlockFace), dfReplacePos, game.Vec32To64(dat.ClickedPosition), c.mPlayer.World())
	case df_world.Block:
		if _, isGlowstone := heldItem.(block.Glowstone); isGlowstone {
			if utils.BlockName(c.mPlayer.World().Block(df_cube.Pos(replacePos))) == "minecraft:respawn_anchor" {
				c.mPlayer.Dbg.Notify(player.DebugModeBlockInteraction, true, "charging respawn anchor with glowstone")
				return true
			}
		}

		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "placing world.Block")

		// If the block at the position is not replacable, we want to place the block on the side of the block.
		if replaceable, ok := replacingBlock.(block.Replaceable); !ok || !replaceable.ReplaceableBy(heldItem) {
			replacePos = replacePos.Side(cube.Face(dat.BlockFace))
		}
		c.mPlayer.PlaceBlock(df_cube.Pos(replacePos), heldItem, nil)
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

	if c.prevPlaceRequest != nil && dat.BlockRuntimeID == c.prevPlaceRequest.BlockRuntimeID && dat.BlockFace == c.prevPlaceRequest.BlockFace &&
		dat.BlockPosition == c.prevPlaceRequest.BlockPosition && dat.HotBarSlot == c.prevPlaceRequest.HotBarSlot &&
		dat.Position == c.prevPlaceRequest.Position && dat.ClickedPosition == c.prevPlaceRequest.ClickedPosition {
		return false
	}

	if c.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) && dat.ClientPrediction != protocol.ClientPredictionSuccess {
		return false
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
	var (
		checkedPositions = make(map[df_cube.Pos]struct{})
		iterCount        int
	)

	for intersectingBlockPos := range game.BlocksBetween(closestEyePos, interactPos) {
		iterCount++
		if iterCount > 49 {
			c.mPlayer.Log().Debug("too many iterations for interaction validation", "eyePos", closestEyePos, "interactPos", interactPos, "checkedPositions", len(checkedPositions))
			break
		}

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

// SetChunkRadius sets the chunk radius of the world updater component.
func (c *WorldUpdaterComponent) SetChunkRadius(radius int32) {
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
