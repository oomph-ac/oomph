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

	chunkRadius       int32
	serverChunkRadius int32

	clientPlacedBlocks  map[df_cube.Pos]*chainedBlockPlacement
	pendingBlockUpdates map[df_cube.Pos]uint32
	batchedBlockUpdates *acknowledgement.UpdateBlockBatch

	breakingBlockPos *protocol.BlockPos
	prevPlaceRequest *protocol.UseItemTransactionData

	initalInteractionAccepted bool
}

func NewWorldUpdaterComponent(p *player.Player) *WorldUpdaterComponent {
	return &WorldUpdaterComponent{
		mPlayer:     p,
		chunkRadius: 1_000_000_000,

		clientPlacedBlocks:  make(map[df_cube.Pos]*chainedBlockPlacement),
		pendingBlockUpdates: make(map[df_cube.Pos]uint32),
		batchedBlockUpdates: acknowledgement.NewUpdateBlockBatchACK(p),

		initalInteractionAccepted: true,
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
	blockBBoxes := utils.BlockCollisions(replacingBlock, clickedBlockPos, c.mPlayer.World())
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
	c.mPlayer.Dbg.Notify(player.DebugModeBlockPlacement, true, "item in hand: %T", heldItem)
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
	if len(utils.BlockCollisions(interactedBlock, blockPos, c.mPlayer.World())) == 0 {
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

		iBBs := utils.BlockCollisions(intersectingBlock, cube.Pos(flooredPos), c.mPlayer.World())
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
			return size, anyConfirmed
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
