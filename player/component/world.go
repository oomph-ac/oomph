package component

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
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

// noinspection ALL
//
//go:linkname world_finaliseBlockRegistry github.com/df-mc/dragonfly/server/world.finaliseBlockRegistry
func world_finaliseBlockRegistry()

func init() {
	world_finaliseBlockRegistry()
}

// WorldUpdaterComponent is a component that handles block and chunk updates to the world of the member player.
type WorldUpdaterComponent struct {
	mPlayer          *player.Player
	breakingBlockPos *protocol.BlockPos
	chunkRadius      int32

	prevPlaceRequest          *protocol.UseItemTransactionData
	initalInteractionAccepted bool
}

func NewWorldUpdaterComponent(p *player.Player) *WorldUpdaterComponent {
	return &WorldUpdaterComponent{
		mPlayer:     p,
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
	acknowledgement.NewChunkUpdateACK(c.mPlayer, pk).Run()
}

// AttemptBlockPlacement attempts a block placement request from the client. It returns false if the simulation is unable
// to place a block at the given position.
func (c *WorldUpdaterComponent) AttemptBlockPlacement(pk *packet.InventoryTransaction) bool {
	dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}

	/* if !c.validateInteraction(pk) {
		return false
	} */
	c.prevPlaceRequest = dat

	// Validate action type.
	if dat.ActionType != protocol.UseItemActionClickBlock {
		return true
	}

	// No item in hand.
	if dat.HeldItem.Stack.NetworkID == 0 {
		return true
	}

	// BlockRuntimeIDs should be positive.
	if dat.HeldItem.Stack.BlockRuntimeID <= 0 {
		return true
	}

	b, ok := df_world.BlockByRuntimeID(uint32(dat.HeldItem.Stack.BlockRuntimeID))
	if !ok {
		return true
	}

	// Find the replace position of the block. This will be used if the block at the current position
	// is replacable (e.g: water, lava, air).
	replacePos := utils.BlockToCubePos(dat.BlockPosition)
	fb := c.mPlayer.World.Block(df_cube.Pos(replacePos))

	// If the block at the position is not replacable, we want to place the block on the side of the block.
	if replaceable, ok := fb.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
		replacePos = replacePos.Side(cube.Face(dat.BlockFace))
	}

	// Make a list of BBoxes the block will occupy.
	boxes := utils.BlockBoxes(b, replacePos, c.mPlayer.World)
	for index, blockBox := range boxes {
		boxes[index] = blockBox.Translate(replacePos.Vec3())
	}

	// Get the player's AABB and translate it to the position of the player. Then check if it intersects
	// with any of the boxes the block will occupy. If it does, we don't want to place the block.
	if cube.AnyIntersections(boxes, c.mPlayer.Movement().BoundingBox()) {
		c.mPlayer.SyncWorld()
		return false
	}

	// Check if any entity is in the way of the block being placed.
	for _, e := range c.mPlayer.EntityTracker().All() {
		if cube.AnyIntersections(boxes, e.Box(e.Position)) {
			c.mPlayer.SyncWorld()
			return false
		}
	}

	c.mPlayer.World.SetBlock(df_cube.Pos(replacePos), b, nil)
	return true
}

func (c *WorldUpdaterComponent) validateInteraction(pk *packet.InventoryTransaction) bool {
	if gamemode := c.mPlayer.GameMode; gamemode != packet.GameTypeSurvival && gamemode != packet.GameTypeAdventure {
		return true
	}

	dat := pk.TransactionData.(*protocol.UseItemTransactionData)
	if dat.ActionType != protocol.UseItemActionClickBlock || dat.ClickedPosition.Len() > 1 { // No point in validating an air click...
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
		isInitalInput = dat.ClickedPosition.LenSqr() > 0
	}

	if !isInitalInput {
		return c.initalInteractionAccepted
	}

	blockPos := cube.Pos{int(dat.BlockPosition.X()), int(dat.BlockPosition.Y()), int(dat.BlockPosition.Z())}
	interactedBlock := c.mPlayer.World.Block(df_cube.Pos(blockPos))
	interactPos := blockPos.Vec3().Add(dat.ClickedPosition)

	if len(utils.BlockBoxes(interactedBlock, blockPos, c.mPlayer.World)) == 0 {
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
	for _, intersectingBlockPos := range game.BlocksBetween(eyePos, interactPos) {
		flooredPos := df_cube.Pos{int(intersectingBlockPos[0]), int(intersectingBlockPos[1]), int(intersectingBlockPos[2])}
		if flooredPos == df_cube.Pos(blockPos) {
			continue
		}

		intersectingBlock := c.mPlayer.World.Block(flooredPos)
		iBBs := utils.BlockBoxes(intersectingBlock, cube.Pos(flooredPos), c.mPlayer.World)
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

// SetBlockBreakPos sets the block breaking pos of the world updater component.
func (c *WorldUpdaterComponent) SetBlockBreakPos(pos *protocol.BlockPos) {
	c.breakingBlockPos = pos
}

// BlockBreakPos returns the block breaking pos of the world updater component.
func (c *WorldUpdaterComponent) BlockBreakPos() *protocol.BlockPos {
	return c.breakingBlockPos
}
