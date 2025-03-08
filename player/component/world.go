package component

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
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

	deferredChunks map[protocol.ChunkPos]*chunk.Chunk
	pendingChunks  map[protocol.ChunkPos]struct{}

	breakingBlockPos          *protocol.BlockPos
	prevPlaceRequest          *protocol.UseItemTransactionData
	chunkRadius               int32
	initalInteractionAccepted bool
}

func NewWorldUpdaterComponent(p *player.Player) *WorldUpdaterComponent {
	return &WorldUpdaterComponent{
		mPlayer: p,

		deferredChunks: make(map[protocol.ChunkPos]*chunk.Chunk),
		pendingChunks:  make(map[protocol.ChunkPos]struct{}),

		chunkRadius: 1024,
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
	acknowledgement.NewChunkUpdateACK(c.mPlayer, pk)
}

// HandleUpdateBlock handles an UpdateBlock packet from the server.
func (c *WorldUpdaterComponent) HandleUpdateBlock(pk *packet.UpdateBlock) {
	pos := df_cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
	b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
	if !ok {
		c.mPlayer.Log().Errorf("unable to find block with runtime ID %v", pk.NewBlockRuntimeID)
		b = block.Air{}
	}

	// TODO: Add a block policy to allow servers to determine wether block updates should be lag-compensated or if movement should
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
	if dat.ActionType != protocol.UseItemActionClickBlock || dat.HeldItem.Stack.NetworkID == 0 || dat.HeldItem.Stack.BlockRuntimeID == 0 {
		return true
	}

	replacePos := utils.BlockToCubePos(dat.BlockPosition)
	dfReplacePos := df_cube.Pos(replacePos)
	replacingBlock := c.mPlayer.WorldTx().Block(dfReplacePos)

	// Ignore the potential block placement if the player clicked air.
	if _, isAir := replacingBlock.(block.Air); isAir {
		return true
	} else if _, ok := replacingBlock.(block.Activatable); ok && !c.mPlayer.Movement().PressingSneak() {
		return true
	}

	heldItem := c.mPlayer.Inventory().Holding().Item()
	switch heldItem := heldItem.(type) {
	case nil:
		// The player has nothing in this slot, ignore the block placement.
		c.mPlayer.NMessage("<red>Block placement denied: no item in hand.</red>")
		return true
	case item.UsableOnBlock:
		useCtx := item.UseContext{}
		heldItem.UseOnBlock(dfReplacePos, df_cube.Face(dat.BlockFace), game.Vec32To64(dat.ClickedPosition), c.mPlayer.WorldTx(), c.mPlayer, &useCtx)
	case world.Block:
		// If the block at the position is not replacable, we want to place the block on the side of the block.
		if replaceable, ok := replacingBlock.(block.Replaceable); !ok || !replaceable.ReplaceableBy(heldItem) {
			replacePos = replacePos.Side(cube.Face(dat.BlockFace))
		}
		c.mPlayer.PlaceBlock(df_cube.Pos(replacePos), heldItem, nil)
	}
	return true
}

func (c *WorldUpdaterComponent) ValidateInteraction(pk *packet.InventoryTransaction) bool {
	if gamemode := c.mPlayer.GameMode; gamemode != packet.GameTypeSurvival && gamemode != packet.GameTypeAdventure {
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

	// On newer versions of the game (1.21.20+), we are able to determine wether the input was from a
	// simulation frame, or from the player itself. However, on older versions there's no other way to
	// distinguish this besides a zero-vector click position that is usually from jump-bridging.
	var isInitalInput bool
	if c.mPlayer.Conn().Proto().ID() >= player.GameVersion1_21_20 {
		isInitalInput = dat.TriggerType == protocol.TriggerTypePlayerInput
	} else {
		isInitalInput = dat.ClickedPosition[0] > 1 || dat.ClickedPosition[1] > 1 || dat.ClickedPosition[2] > 1
	}

	if !isInitalInput {
		return c.initalInteractionAccepted
	}

	blockPos := cube.Pos{int(dat.BlockPosition.X()), int(dat.BlockPosition.Y()), int(dat.BlockPosition.Z())}
	interactedBlock := c.mPlayer.WorldTx().Block(df_cube.Pos(blockPos))
	interactPos := blockPos.Vec3().Add(dat.ClickedPosition)

	if len(utils.BlockBoxes(interactedBlock, blockPos, c.mPlayer.WorldTx())) == 0 {
		c.initalInteractionAccepted = true
		return true
	}

	eyePos := c.mPlayer.Movement().Pos()
	if c.mPlayer.Movement().Sneaking() {
		eyePos[1] += 1.54
	} else {
		eyePos[1] += 1.62
	}

	if eyePos.Sub(interactPos).Len() >= 7.0 {
		c.initalInteractionAccepted = false
		c.mPlayer.NMessage("<red>Interaction denied: too far away.</red>")
		return false
	}

	// Check for all the blocks in between the interaction position and the player's eye position. If any blocks intersect
	// with the line between the player's eye position and the interaction position, the interaction is cancelled.
	for intersectingBlockPos := range game.BlocksBetween(eyePos, interactPos) {
		flooredPos := df_cube.Pos{int(intersectingBlockPos[0]), int(intersectingBlockPos[1]), int(intersectingBlockPos[2])}
		if flooredPos == df_cube.Pos(blockPos) {
			continue
		}

		intersectingBlock := c.mPlayer.WorldTx().Block(flooredPos)
		iBBs := utils.BlockBoxes(intersectingBlock, cube.Pos(flooredPos), c.mPlayer.WorldTx())
		if len(iBBs) == 0 {
			continue
		}

		// Iterate through all the block's bounding boxes to check if it is in the way of the interaction position.
		for _, iBB := range iBBs {
			iBB = iBB.Translate(intersectingBlockPos)

			// If there is an intersection, the interaction is invalid.
			if _, ok := trace.BBoxIntercept(iBB, eyePos, interactPos); ok {
				c.mPlayer.NMessage("<red>Interaction denied: block obstructs path.</red>")
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

func (c *WorldUpdaterComponent) DeferChunk(pos protocol.ChunkPos, chunk *chunk.Chunk) {
	delete(c.pendingChunks, pos)
	c.deferredChunks[pos] = chunk
}

func (c *WorldUpdaterComponent) ChunkDeferred(pos protocol.ChunkPos) (*chunk.Chunk, bool) {
	chunk, ok := c.deferredChunks[pos]
	return chunk, ok
}

func (c *WorldUpdaterComponent) ChunkPending(pos protocol.ChunkPos) bool {
	_, isChunkPending := c.pendingChunks[pos]
	return isChunkPending
}

func (c *WorldUpdaterComponent) GenerateChunk(pos df_world.ChunkPos, chunk *chunk.Chunk) {
	c.pendingChunks[protocol.ChunkPos(pos)] = struct{}{}
}

// SetBlockBreakPos sets the block breaking pos of the world updater component.
func (c *WorldUpdaterComponent) SetBlockBreakPos(pos *protocol.BlockPos) {
	c.breakingBlockPos = pos
}

// BlockBreakPos returns the block breaking pos of the world updater component.
func (c *WorldUpdaterComponent) BlockBreakPos() *protocol.BlockPos {
	return c.breakingBlockPos
}

func (w *WorldUpdaterComponent) Tick() {
	for pos, c := range w.deferredChunks {
		worldColumn, loaded := w.mPlayer.WorldLoader().Chunk(df_world.ChunkPos(pos))
		if loaded {
			worldColumn.Chunk = c
			delete(w.deferredChunks, pos)
		}
	}
}
