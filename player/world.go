package player

import (
	"math"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// WorldUpdaterComponent is a component that handles block and chunk updates to the world of the member player.
type WorldUpdaterComponent interface {
	// HandleLevelChunk allows the world updater component to handle a LevelChunk packet sent by the server.
	HandleLevelChunk(pk *packet.LevelChunk)
	// HandleSubChunk allows the world updater component to handle a SubChunk packet sent by the server.
	HandleSubChunk(pk *packet.SubChunk)
	// HandleUpdateBlock allows the world updater component to handle an UpdateBlock packet sent by the server.
	HandleUpdateBlock(pk *packet.UpdateBlock)
	// HandleUpdateSubChunkBlocks allows the world updater component to handle a UpdateSubChunkBlocks packet sent by the server.
	HandleUpdateSubChunkBlocks(pk *packet.UpdateSubChunkBlocks)
	// AttemptItemInteractionWithBlock attempts an item interaction with a block request from the client. It returns false if the simulation is unable
	// to place a block at the given position.
	AttemptItemInteractionWithBlock(pk *packet.InventoryTransaction) bool
	// ValidateInteraction validates if a player is allowed to perform an action on an interactable block.
	ValidateInteraction(pk *packet.InventoryTransaction) bool

	// SetServerChunkRadius sets the server chunk radius of the world updater component.
	SetServerChunkRadius(radius int32)
	// SetChunkRadius sets the chunk radius of the world updater component.
	SetChunkRadius(radius int32)
	// ChunkRadius returns the chunk radius of the world updater component.
	ChunkRadius() int32

	// SetBlockBreakPos sets the block breaking pos of the world updater component.
	SetBlockBreakPos(pos *protocol.BlockPos)
	// BlockBreakPos returns the block breaking pos of the world updater component.
	BlockBreakPos() *protocol.BlockPos

	// QueueBlockPlacement queues a block placement.
	QueueBlockPlacement(clickedBlockPos, placedBlockPos df_cube.Pos, face df_cube.Face)
	// AddPendingUpdate adds a pending block update.
	AddPendingUpdate(pos df_cube.Pos, blockRuntimeID uint32)
	// HasPendingUpdate checks if a block update is pending at the given block position.
	HasPendingUpdate(pos df_cube.Pos) bool
	// RemovePendingUpdate removes a pending block update.
	RemovePendingUpdate(pos df_cube.Pos, blockRuntimeID uint32)

	// Tick ticks the world updater component.
	Tick()
	// Flush flushes the world updater component.
	Flush()
}

func (p *Player) SetWorldUpdater(c WorldUpdaterComponent) {
	p.worldUpdater = c
}

func (p *Player) WorldUpdater() WorldUpdaterComponent {
	return p.worldUpdater
}

func (p *Player) World() *oworld.World {
	return p.world
}

// This function is deprecated and instead, the user should call p.World().PurgeChunks() directly.
func (p *Player) RegenerateWorld() {
	p.world.PurgeChunks()
}

func (p *Player) SyncWorld() {
	// Update the blocks in the world so the client can sync itself properly.
	pos := p.Movement().Pos()
	for x := int(math32.Floor(pos[0] - 0.05)); x <= int(pos[0]+0.05); x++ {
		for y := int(math32.Floor(pos[1] - 0.05)); y <= int(pos[1]+0.05); y++ {
			for z := int(math32.Floor(pos[2] - 0.05)); z <= int(pos[2]+0.05); z++ {
				p.SyncBlock(df_cube.Pos{x, y, z})
			}
		}
	}
}

func (p *Player) SyncBlock(pos df_cube.Pos) {
	// Avoid syncing blocks when there is a pending update already for that block - it can cause a desync.
	if p.WorldUpdater().HasPendingUpdate(pos) {
		return
	}
	pk := &packet.UpdateBlock{
		Position: protocol.BlockPos{
			int32(pos[0]),
			int32(pos[1]),
			int32(pos[2]),
		},
		NewBlockRuntimeID: world.BlockRuntimeID(p.World().Block(pos)),
		Flags:             packet.BlockUpdateNetwork,
		Layer:             0, // TODO: Implement and account for multi-layer blocks.
	}
	p.WorldUpdater().HandleUpdateBlock(pk)
	_ = p.SendPacketToClient(pk)
}

func (p *Player) PlaceBlock(clickedBlockPos, replaceBlockPos df_cube.Pos, face df_cube.Face, b world.Block) {
	replacingBlock := p.World().Block(replaceBlockPos)
	if _, isReplaceable := replacingBlock.(block.Replaceable); !isReplaceable {
		p.Dbg.Notify(DebugModeBlockPlacement, true, "block at %v (%T) is not replaceable", replaceBlockPos, replacingBlock)
		return
	}

	// Make a list of BBoxes the block will occupy.
	boxes := utils.BlockCollisions(b, cube.Pos(replaceBlockPos), p.World())
	for index, blockBox := range boxes {
		boxes[index] = blockBox.Translate(cube.Pos(replaceBlockPos).Vec3())
	}

	// Get the player's AABB and translate it to the position of the player. Then check if it intersects
	// with any of the boxes the block will occupy. If it does, we don't want to place the block.
	if cube.AnyIntersections(boxes, p.Movement().BoundingBox()) && !utils.CanPassBlock(b) {
		p.SyncBlock(replaceBlockPos)
		p.Inventory().ForceSync()
		p.Dbg.Notify(DebugModeBlockPlacement, true, "player AABB intersects with block at %v", replaceBlockPos)
		return
	}

	// Check if any entity is in the way of the block being placed.
	entityIntersecting := false
	if p.Opts().Combat.EnableClientEntityTracking {
		for _, e := range p.ClientEntityTracker().All() {
			if cube.AnyIntersections(boxes, e.Box(e.Position)) {
				entityIntersecting = true
				break
			}
		}
	} else {
		for _, e := range p.EntityTracker().All() {
			if rew, ok := e.Rewind(p.ClientTick); ok && cube.AnyIntersections(boxes, e.Box(rew.Position)) {
				entityIntersecting = true
				break
			}
		}
	}

	if entityIntersecting {
		// We sync the world in this instance to avoid any possibility of a long-persisting ghost block.
		p.SyncBlock(replaceBlockPos)
		p.Inventory().ForceSync()
		p.Dbg.Notify(DebugModeBlockPlacement, true, "entity prevents block placement at %v", replaceBlockPos)
		return
	}

	inv, _ := p.inventory.WindowFromWindowID(protocol.WindowIDInventory)
	inv.SetSlot(int(p.inventory.HeldSlot()), p.inventory.Holding().Grow(-1))
	p.WorldUpdater().QueueBlockPlacement(clickedBlockPos, replaceBlockPos, face)
	p.World().SetBlock(replaceBlockPos, b, nil)
	p.Dbg.Notify(DebugModeBlockPlacement, true, "placed block at %v", replaceBlockPos)
}

func (p *Player) SendBlockUpdates(positions []protocol.BlockPos) {
	for _, pos := range positions {
		p.SendPacketToClient(&packet.UpdateBlock{
			Position: pos,
			NewBlockRuntimeID: world.BlockRuntimeID(p.World().Block(df_cube.Pos{
				int(pos.X()),
				int(pos.Y()),
				int(pos.Z()),
			})),
			Flags: packet.BlockUpdateNeighbours,
			Layer: 0, // TODO: Implement and account for multi-layer blocks.
		})
	}
}

func (p *Player) handleBlockActions(pk *packet.PlayerAuthInput) {
	/* chunkPos := protocol.ChunkPos{
		int32(math32.Floor(p.movement.Pos().X())) >> 4,
		int32(math32.Floor(p.movement.Pos().Z())) >> 4,
	} */

	var (
		handledBlockBreak             bool
		isFullServerAuthBlockBreaking = p.ServerConn() == nil || p.GameDat.PlayerMovementSettings.ServerAuthoritativeBlockBreaking
	)
	if blockBreakPos := p.worldUpdater.BlockBreakPos(); blockBreakPos != nil && p.blockBreakInProgress && isFullServerAuthBlockBreaking {
		p.blockBreakProgress += 1.0 / math32.Max(p.expectedBlockBreakTime(*blockBreakPos), 0.001)
		p.Dbg.Notify(DebugModeBlockBreaking, true, "(handleBlockActions) assuming block break in progress (blockBreakProgress=%.4f)", p.blockBreakProgress)
		handledBlockBreak = true
	}

	if pk.InputData.Load(packet.InputFlagPerformBlockActions) {
		newActions := make([]protocol.PlayerBlockAction, 0, len(pk.BlockActions))
		for _, action := range pk.BlockActions {
			p.Dbg.Notify(DebugModeBlockBreaking, true, "blockAction=%v", action)
			switch action.Action {
			case protocol.PlayerActionPredictDestroyBlock:
				if !isFullServerAuthBlockBreaking || p.worldUpdater.BlockBreakPos() == nil {
					p.Dbg.Notify(
						DebugModeBlockBreaking,
						true,
						"ignored PlayerActionPredictDestroyBlock (isFullServerAuthBlockBreaking=%v, blockBreakPos=%v)",
						isFullServerAuthBlockBreaking,
						p.worldUpdater.BlockBreakPos(),
					)
					continue
				}

				if !p.tryBreakBlock(cube.Face(action.Face)) {
					continue
				}
				p.blockBreakProgress = 0.0
				p.blockBreakInProgress = false
				p.World().SetBlock(df_cube.Pos{
					int(action.BlockPos.X()),
					int(action.BlockPos.Y()),
					int(action.BlockPos.Z()),
				}, block.Air{}, nil)
				p.Dbg.Notify(DebugModeBlockBreaking, true, "(PlayerActionPredictDestroyBlock) broke block at %v", action.BlockPos)
			case protocol.PlayerActionStartBreak, protocol.PlayerActionCrackBreak:
				if action.Action == protocol.PlayerActionStartBreak {
					// We assume a potential mispredction here because the client while clicking, think it may need to break
					// a block, but the server may instead think an entity is in the way of that block, constituting
					// a misprediction.
					if p.InputMode != packet.InputModeTouch {
						p.combat.Attack(nil)
					}
					// In this scenario, the client should be trying to continue breaking a block, which means the one they targeted
					// previously was broken.
					p.blockBreakProgress = 0.0
					p.blockBreakInProgress = true
					p.blockBreakStartTick = p.ClientTick
					p.Dbg.Notify(DebugModeBlockBreaking, true, "(PlayerActionStartBreak) started breaking block at %v", action.BlockPos)
				}

				currentBlockBreakPos := p.worldUpdater.BlockBreakPos()
				if currentBlockBreakPos == nil || *currentBlockBreakPos != action.BlockPos {
					p.Dbg.Notify(DebugModeBlockBreaking, true, "reset block break progress (currentBlockBreakPos=%v, action.BlockPos=%v)", currentBlockBreakPos, action.BlockPos)
					p.blockBreakProgress = 0.0
					p.blockBreakStartTick = p.ClientTick
				}
				p.blockBreakProgress += 1.0 / math32.Max(p.expectedBlockBreakTime(action.BlockPos), 0.001)
				p.worldUpdater.SetBlockBreakPos(&action.BlockPos)
			case protocol.PlayerActionContinueDestroyBlock:
				if currentBreakPos := p.worldUpdater.BlockBreakPos(); !p.blockBreakInProgress || (currentBreakPos != nil && *currentBreakPos != action.BlockPos) {
					p.blockBreakProgress = 0.0
					p.blockBreakStartTick = p.ClientTick
					p.Dbg.Notify(DebugModeBlockBreaking, true, "different block being broken - reset block break progress (currentBreakPos=%v, action.BlockPos=%v)", currentBreakPos, action.BlockPos)
				}
				p.blockBreakProgress += 1.0 / math32.Max(p.expectedBlockBreakTime(action.BlockPos), 0.001)
				p.worldUpdater.SetBlockBreakPos(&action.BlockPos)
				p.blockBreakInProgress = true

				// We assume a potential mispredction here because the client while clicking, think it may need to break
				// a block, but the server may instead think an entity is in the way of that block, constituting
				// a misprediction.
				if p.InputMode != packet.InputModeTouch {
					p.combat.Attack(nil)
				}
			case protocol.PlayerActionAbortBreak:
				//p.Message("abort break")
				p.blockBreakProgress = 0.0
				p.Dbg.Notify(DebugModeBlockBreaking, true, "aborted block break (currentBreakPos=%v)", p.worldUpdater.BlockBreakPos())
				p.worldUpdater.SetBlockBreakPos(nil)
				p.blockBreakInProgress = false
			case protocol.PlayerActionStopBreak:
				if p.worldUpdater.BlockBreakPos() == nil {
					p.Dbg.Notify(DebugModeBlockBreaking, true, "ignored PlayerActionStopBreak (blockBreakPos is nil)")
					continue
				}

				if !p.tryBreakBlock(cube.Face(action.Face)) {
					continue
				}
				p.blockBreakProgress = 0.0
				p.blockBreakInProgress = false
				p.World().SetBlock(df_cube.Pos{
					int(p.worldUpdater.BlockBreakPos().X()),
					int(p.worldUpdater.BlockBreakPos().Y()),
					int(p.worldUpdater.BlockBreakPos().Z()),
				}, block.Air{}, nil)
				p.Dbg.Notify(DebugModeBlockBreaking, true, "(PlayerActionStopBreak) broke block at %v", p.worldUpdater.BlockBreakPos())
			}
			newActions = append(newActions, action)
		}
		pk.BlockActions = newActions
		if len(newActions) == 0 {
			pk.InputData.Unset(packet.InputFlagPerformBlockActions)
		}
	}

	if !handledBlockBreak && isFullServerAuthBlockBreaking {
		if blockBreakPos := p.worldUpdater.BlockBreakPos(); blockBreakPos != nil && p.blockBreakInProgress {
			p.blockBreakProgress += 1.0 / math32.Max(p.expectedBlockBreakTime(*blockBreakPos), 0.001)
			p.Dbg.Notify(DebugModeBlockBreaking, true, "(afterTheFactHack) block break in progress (blockBreakProgress=%.4f)", p.blockBreakProgress)
		}
	}

	/* if p.Ready {
		p.World().CleanChunks(p.worldUpdater.ChunkRadius(), chunkPos)
	} */
}

func (p *Player) blockInteractable(blockPos cube.Pos, interactFace cube.Face) bool {
	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return true
	}

	// If the player is a non-touch player, we should just do a raycast and make sure no blocks are in the way. However, the check
	// below is faster and really the only way we can account for touch players anyway.
	/* if p.InputMode != packet.InputModeTouch {
		return p.tryRaycastToBlock(blockPos)
	} */

	interactableFaces := make(map[cube.Face]struct{}, 6)
	prevPos := cube.PosFromVec3(p.Movement().Pos().Add(mgl32.Vec3{0, game.DefaultPlayerHeightOffset, 0}))
	currPos := cube.PosFromVec3(p.Movement().Pos().Add(mgl32.Vec3{0, game.DefaultPlayerHeightOffset, 0}))
	blockX, blockY, blockZ := blockPos[0], blockPos[1], blockPos[2]

	// If the player's head is inside the block they are breaking, allow them to break it.
	if prevPos[1] == blockY || currPos[1] == blockY {
		return true
	}

	belowBlock := currPos[1] < blockY || prevPos[1] < blockY
	aboveBlock := currPos[1] > blockY || prevPos[1] > blockY
	if belowBlock {
		interactableFaces[cube.FaceDown] = struct{}{}
	}
	if aboveBlock {
		interactableFaces[cube.FaceUp] = struct{}{}
	}
	if currPos[0] < blockX || prevPos[0] < blockX {
		interactableFaces[cube.FaceWest] = struct{}{}
	}
	if currPos[0] > blockX || prevPos[0] > blockX {
		interactableFaces[cube.FaceEast] = struct{}{}
	}
	if currPos[2] < blockZ || prevPos[2] < blockZ {
		interactableFaces[cube.FaceNorth] = struct{}{}
	}
	if currPos[2] > blockZ || prevPos[2] > blockZ {
		interactableFaces[cube.FaceSouth] = struct{}{}
	}

	if _, ok := interactableFaces[interactFace]; !ok {
		p.Dbg.Notify(DebugModeBlockBreaking, true, "interactFace=%v is not interactable (currPos=%v, prevPos=%v, blockPos=%v)", interactFace, currPos, prevPos, blockPos)
		return false
	}

	for face := range interactableFaces {
		sidePos := blockPos.Side(face)
		sideBlock := p.World().Block([3]int(sidePos))
		sideBBs := utils.BlockCollisions(sideBlock, sidePos, p.World())

		// There are no bounding boxes in the way of this face, we can interact
		if len(sideBBs) == 0 {
			return true
		}
	bb_check:
		for _, indexBB := range sideBBs {
			if (indexBB.Width() == 1 || indexBB.Length() == 1) && indexBB.Height() == 1 {
				// Continue to search in other faces for free areas.
				break bb_check
			} else {
				return true
			}
		}
	}
	return false
}

// tryRaycastToBlock runs a raycast to the block at the given position. This function assumes that to break the target block, the
// player must be aiming at it at all times, so we just use the previous position & current rotation (movement component
// isn't updated yet when block actions are handled) instead of trying to account for frame action shitfuckery.
//
// It returns true if:
// 1. The raycast hits, and is within the max interaction distance.
// 2. There are no blocks in the way of the raycast.
// OR: if the player is not in survival or adventure mode.
func (p *Player) tryRaycastToBlock(blockPos cube.Pos) bool {
	rotation := p.Movement().LastRotation()
	eyeHeight := game.DefaultPlayerHeightOffset
	if p.Movement().Sneaking() {
		eyeHeight = game.SneakingPlayerHeightOffset
	}

	raycastDir := game.DirectionVector(rotation.Z(), rotation.X())
	raycastStart := p.Movement().LastPos().Add(mgl32.Vec3{0, eyeHeight, 0})
	raycastEnd := raycastStart.Add(raycastDir.Mul(game.MaxBlockInteractionDistance))

	brokenBlockBB := cube.Box(0, 0, 0, 1, 1, 1).Translate(blockPos.Vec3())
	raycast, ok := trace.BBoxIntercept(brokenBlockBB, raycastStart, raycastEnd)
	if !ok {
		return false
	}
	hitPos := raycast.Position()
	for intersectingBlockPos := range game.BlocksBetween(raycastStart, hitPos, 128) {
		flooredPos := cube.PosFromVec3(intersectingBlockPos)
		if flooredPos == blockPos {
			continue
		}
		intersectingBlock := p.World().Block([3]int(flooredPos))
		for _, intersectingBlockBB := range utils.BlockCollisions(intersectingBlock, flooredPos, p.World()) {
			intersectingBlockBB = intersectingBlockBB.Translate(intersectingBlockPos)
			if _, ok := trace.BBoxIntercept(intersectingBlockBB, raycastStart, raycastEnd); ok {
				return false
			}
		}
	}
	return true
}

func (p *Player) tryBreakBlock(interactFace cube.Face) bool {
	breakPosPtr := p.worldUpdater.BlockBreakPos()
	if breakPosPtr == nil {
		p.Dbg.Notify(DebugModeBlockBreaking, true, "ignored PlayerActionStopBreak (blockBreakPos is nil)")
		return false
	}

	breakPos := *breakPosPtr
	p.blockBreakProgress += 1.0 / math32.Max(p.expectedBlockBreakTime(breakPos), 0.001)
	p.Dbg.Notify(DebugModeBlockBreaking, true, "block break progress=%.4f", p.blockBreakProgress)
	if p.blockBreakProgress <= 0.999 {
		p.SendBlockUpdates([]protocol.BlockPos{breakPos})
		p.Popup("<red>Broke block too early!</red>")
		p.Dbg.Notify(DebugModeBlockBreaking, true, "cancelled break action (blockBreakProgress=%.4f)", p.blockBreakProgress)
		p.blockBreakProgress = 0.0
		return false
	}
	if !p.blockInteractable(cube.Pos{int(breakPos[0]), int(breakPos[1]), int(breakPos[2])}, interactFace) {
		p.SendBlockUpdates([]protocol.BlockPos{breakPos})
		p.Popup("<red>Cannot break this block!</red>")
		p.blockBreakProgress = 0.0
		return false
	}
	return true
}

func (p *Player) expectedBlockBreakTime(pos protocol.BlockPos) float32 {
	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return 0
	}

	held, _ := p.HeldItems()
	/* if _, isShear := held.Item().(item.Shears); isShear {
		// FFS???
		return 1
	} */
	p.Dbg.Notify(DebugModeBlockBreaking, true, "itemInHand=%v", held)

	// FIXME: It seems like Dragonfly doesn't have the item runtime IDs for Netherite tools set properly. This is a temporary
	// hack to allow netherite tools to work. However, it introduces a bypass where any nethite tool will be able to break
	// blocks instantly.
	// NOTE: This was removed because we use the items the server sends us, instead of relying on Dragonfly's 100%.
	/* if strings.Contains(utils.ItemName(held.Item()), "netherite") {
		return 0
	} */

	b := p.World().Block(df_cube.Pos{int(pos.X()), int(pos.Y()), int(pos.Z())})
	if hash1, hash2 := b.Hash(); hash1 == 0 && hash2 == math.MaxUint64 {
		// If the block hash is MaxUint64, then the block is unknown to dragonfly. In the future,
		// we should implement more blocks to avoid this condition allowing clients to break those
		// blocks at any interval they please.
		return 0
	}

	if _, isAir := b.(block.Air); isAir {
		// Let the player send a break action for air, it won't affect anything in-game.
		return 0
	} else if utils.BlockName(b) == "minecraft:web" {
		// Cobwebs are not implemented in Dragonfly, and therefore the break time duration won't be accurate.
		// Just return 1 and accept when the client does break the cobweb.
		return 1
	} else if utils.BlockName(b) == "minecraft:bed" {
		return 1
	}

	breakTime := float32(block.BreakDuration(b, held).Milliseconds())
	// On versions below 1.21.50, the block break time for wool is shorter by ~25% See https://github.com/oomph-ac/oomph/issues/107
	if _, isWool := b.(block.Wool); isWool && p.Version < GameVersion1_21_50 {
		breakTime *= 0.75
	}

	/* if !p.movement.OnGround() {
		breakTime *= 5
	} */
	for effectID, e := range p.effects.All() {
		switch effectID {
		case packet.EffectHaste:
			breakTime *= float32(1 - (0.2 * float64(e.Amplifier)))
		case packet.EffectMiningFatigue:
			breakTime *= float32(1 + (0.3 * float64(e.Amplifier)))
		}
	}
	return float32(breakTime / 50)
}
