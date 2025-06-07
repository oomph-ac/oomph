package component

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
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
		return
	}
	acknowledgement.NewChunkUpdateACK(c.mPlayer, pk).Run()
}

// HandleUpdateBlock handles an UpdateBlock packet from the server.
func (c *WorldUpdaterComponent) HandleUpdateBlock(pk *packet.UpdateBlock) {
	pos := df_cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
	b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
	if !ok {
		c.mPlayer.Log().Errorf("unable to find block with runtime ID %v", pk.NewBlockRuntimeID)
		b = block.Air{}
	}

	// TODO: Add a block policy to allow servers to determine whether block updates should be lag-compensated or if movement should
	// use the latest world state instantly.
	c.mPlayer.ACKs().Add(acknowledgement.NewUpdateBlockACK(c.mPlayer, pos, b))
}

// AttemptBlockPlacement attempts a block placement request from the client. It returns false if the simulation is unable
// to place a block at the given position.
func (c *WorldUpdaterComponent) AttemptBlockPlacement(pk *packet.InventoryTransaction) bool {
	dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}

	c.prevPlaceRequest = dat
	if dat.ActionType != protocol.UseItemActionClickBlock {
		return true
	}

	if c.mPlayer.VersionInRange(player.GameVersion1_21_20, 99999999) {
		if dat.ClientPrediction == protocol.ClientPredictionFailure {
			c.mPlayer.Message("failure :(")
			return false
		}
		c.mPlayer.Message("success :)")
	}

	replacePos := utils.BlockToCubePos(dat.BlockPosition)
	dfReplacePos := df_cube.Pos(replacePos)
	replacingBlock := c.mPlayer.World().Block(dfReplacePos)

	// Ignore the potential block placement if the player clicked air.
	if _, isAir := replacingBlock.(block.Air); isAir {
		return true
	} else if _, ok := replacingBlock.(block.Activatable); ok && !c.mPlayer.Movement().PressingSneak() {
		return true
	}

	heldItem := c.mPlayer.Inventory().Holding().Item()
	c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "item in hand: %T", heldItem)

	switch heldItem := heldItem.(type) {
	case *block.Air:
		// This only happens when Dragonfly is unsure of what the item is (unregistered), so we use the client-authoritative block in hand.
		if b, ok := df_world.BlockByRuntimeID(uint32(dat.HeldItem.Stack.BlockRuntimeID)); ok {
			// If the block at the position is not replacable, we want to place the block on the side of the block.
			if replaceable, ok := replacingBlock.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
				replacePos = replacePos.Side(cube.Face(dat.BlockFace))
			}

			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "using client-authoritative block in hand: %T", b)
			c.mPlayer.PlaceBlock(df_cube.Pos(replacePos), b, nil)
		}
	case nil:
		// The player has nothing in this slot, ignore the block placement.
		// FIXME: It seems some blocks aren't implemented by Dragonfly and will therefore seem to be air when
		// it is actually a valid block.
		//c.mPlayer.NMessage("<red>Block placement denied: no item in hand.</red>")
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "Block placement denied: no item in hand.")
		return true
	case item.UsableOnBlock:
		// TODO: Re-implement all the use block functionality without the use of Dragonfly and world transactions.
		if b, ok := heldItem.(world.Block); ok {
			c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "running interaction w/ item.UsableOnBlock")
			utils.UseOnBlock(c.mPlayer, b, df_cube.Face(dat.BlockFace), dfReplacePos, game.Vec32To64(dat.ClickedPosition), c.mPlayer.World())
			c.mPlayer.Message("yummy bitch")
		}
		/* useCtx := item.UseContext{}
		heldItem.UseOnBlock(dfReplacePos, df_cube.Face(dat.BlockFace), game.Vec32To64(dat.ClickedPosition), c.mPlayer.World(), c.mPlayer, &useCtx) */
	case df_world.Block:
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "world.Block")

		// If the block at the position is not replacable, we want to place the block on the side of the block.
		if replaceable, ok := replacingBlock.(block.Replaceable); !ok || !replaceable.ReplaceableBy(heldItem) {
			replacePos = replacePos.Side(cube.Face(dat.BlockFace))
		}
		c.mPlayer.PlaceBlock(df_cube.Pos(replacePos), heldItem, nil)
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

	// On newer versions of the game (1.21.20+), we are able to determine whether the input was from a
	// simulation frame, or from the player itself. However, on older versions there's no other way to
	// distinguish this besides a zero-vector click position that is usually from jump-bridging.
	var isInitalInput bool
	if c.mPlayer.Conn().Proto().ID() >= player.GameVersion1_21_20 {
		isInitalInput = dat.TriggerType == protocol.TriggerTypePlayerInput
	} else {
		isInitalInput = dat.ClickedPosition.LenSqr() > 0.0 && dat.ClickedPosition.LenSqr() <= 1.0
	}
	if !isInitalInput {
		return c.initalInteractionAccepted
	}

	blockPos := cube.Pos{int(dat.BlockPosition.X()), int(dat.BlockPosition.Y()), int(dat.BlockPosition.Z())}
	interactedBlock := c.mPlayer.World().Block(df_cube.Pos(blockPos))
	interactPos := blockPos.Vec3().Add(dat.ClickedPosition)

	if len(utils.BlockBoxes(interactedBlock, blockPos, c.mPlayer.World())) == 0 {
		c.initalInteractionAccepted = true
		return true
	}

	eyePos := c.mPlayer.Movement().Pos()
	if c.mPlayer.Movement().Sneaking() {
		eyePos[1] += 1.54
	} else {
		eyePos[1] += 1.62
	}

	if dist := eyePos.Sub(interactPos).Len(); dist >= 7.0 {
		c.initalInteractionAccepted = false
		c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "Interaction denied: too far away (%.4f blocks).", dist)
		//c.mPlayer.NMessage("<red>Interaction denied: too far away.</red>")
		return false
	}

	// Check for all the blocks in between the interaction position and the player's eye position. If any blocks intersect
	// with the line between the player's eye position and the interaction position, the interaction is cancelled.
	var (
		checkedPositions = make(map[df_cube.Pos]struct{})
		iterCount        int
	)

	for intersectingBlockPos := range game.BlocksBetween(eyePos, interactPos) {
		iterCount++
		if iterCount > 49 {
			c.mPlayer.Log().Debugf("too many iterations for interaction validation (eyePos=%v interactPos=%v uniqueBlocks=%d)", eyePos, interactPos, len(checkedPositions))
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
		iBBs := utils.BlockBoxes(intersectingBlock, cube.Pos(flooredPos), c.mPlayer.World())
		if len(iBBs) == 0 {
			continue
		}

		// Iterate through all the block's bounding boxes to check if it is in the way of the interaction position.
		for _, iBB := range iBBs {
			iBB = iBB.Translate(intersectingBlockPos)

			// If there is an intersection, the interaction is invalid.
			if _, ok := trace.BBoxIntercept(iBB, eyePos, interactPos); ok {
				//c.mPlayer.NMessage("<red>Interaction denied: block obstructs path.</red>")
				c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "Interaction denied: block obstructs path.")
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
