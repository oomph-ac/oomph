package player

import (
	"math"
	"strings"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
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
	// AttemptItemInteractionWithBlock attempts an item interaction with a block request from the client. It returns false if the simulation is unable
	// to place a block at the given position.
	AttemptItemInteractionWithBlock(pk *packet.InventoryTransaction) bool
	// ValidateInteraction validates if a player is allowed to perform an action on an interactable block.
	ValidateInteraction(pk *packet.InventoryTransaction) bool

	// SetChunkRadius sets the chunk radius of the world updater component.
	SetChunkRadius(radius int32)
	// ChunkRadius returns the chunk radius of the world updater component.
	ChunkRadius() int32

	// SetBlockBreakPos sets the block breaking pos of the world updater component.
	SetBlockBreakPos(pos *protocol.BlockPos)
	// BlockBreakPos returns the block breaking pos of the world updater component.
	BlockBreakPos() *protocol.BlockPos
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
	for _, blockResult := range utils.GetNearbyBlocks(p.Movement().BoundingBox(), true, true, p.World()) {
		p.SendPacketToClient(&packet.UpdateBlock{
			Position: protocol.BlockPos{
				int32(blockResult.Position[0]),
				int32(blockResult.Position[1]),
				int32(blockResult.Position[2]),
			},
			NewBlockRuntimeID: world.BlockRuntimeID(blockResult.Block),
			Flags:             packet.BlockUpdatePriority,
			Layer:             0, // TODO: Implement and account for multi-layer blocks.
		})
	}
}

func (p *Player) PlaceBlock(pos df_cube.Pos, b world.Block, ctx *item.UseContext) {
	replacingBlock := p.World().Block(pos)
	if _, isReplaceable := replacingBlock.(block.Replaceable); !isReplaceable {
		p.Dbg.Notify(DebugModeBlockPlacement, true, "block at %v is not replaceable", pos)
		return
	}

	// Make a list of BBoxes the block will occupy.
	boxes := utils.BlockBoxes(b, cube.Pos(pos), p.World())
	for index, blockBox := range boxes {
		boxes[index] = blockBox.Translate(cube.Pos(pos).Vec3())
	}

	// Get the player's AABB and translate it to the position of the player. Then check if it intersects
	// with any of the boxes the block will occupy. If it does, we don't want to place the block.
	if cube.AnyIntersections(boxes, p.Movement().BoundingBox()) && !utils.CanPassBlock(b) {
		//p.SyncWorld()
		p.Dbg.Notify(DebugModeBlockPlacement, true, "player AABB intersects with block at %v", pos)
		return
	}

	// Check if any entity is in the way of the block being placed.
	for eid, e := range p.EntityTracker().All() {
		rew, ok := e.Rewind(p.ClientTick)
		if !ok {
			continue
		}

		// We sync the world in this instance to avoid any possibility of a long-persisting ghost block.
		if cube.AnyIntersections(boxes, e.Box(rew.Position)) {
			p.SendBlockUpdates([]protocol.BlockPos{
				{int32(pos[0]), int32(pos[1]), int32(pos[2])},
			})
			p.Dbg.Notify(DebugModeBlockPlacement, true, "entity %d intersects with block at %v", eid, pos)
			return
		}
	}

	/* inv, _ := p.inventory.WindowFromWindowID(protocol.WindowIDInventory)
	inv.SetSlot(int(p.inventory.HeldSlot()), p.inventory.Holding().Grow(-1)) */
	p.World().SetBlock(pos, b, nil)
	p.Dbg.Notify(DebugModeBlockPlacement, true, "placed block at %v", pos)
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
		p.blockBreakProgress += 1.0 / math32.Max(p.getExpectedBlockBreakTime(*blockBreakPos), 0.001)
		p.Dbg.Notify(DebugModeBlockBreaking, true, "(handleBlockActions) assuming block break in progress (blockBreakProgress=%.4f)", p.blockBreakProgress)
		handledBlockBreak = true
	}

	if pk.InputData.Load(packet.InputFlagPerformBlockActions) {
		var newActions = make([]protocol.PlayerBlockAction, 0, len(pk.BlockActions))
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

				finalProgress := p.blockBreakProgress + (1.0 / math32.Max(p.getExpectedBlockBreakTime(*p.worldUpdater.BlockBreakPos()), 0.001))
				p.Dbg.Notify(DebugModeBlockBreaking, true, "finalProgress=%.4f", finalProgress)
				if finalProgress <= 0.999 {
					p.SendBlockUpdates([]protocol.BlockPos{*p.worldUpdater.BlockBreakPos()})
					pk.InputData.Unset(packet.InputFlagPerformItemInteraction)
					p.Popup("<red>Broke block too early!</red>")
					p.Dbg.Notify(DebugModeBlockBreaking, true, "cancelled PlayerActionPredictDestroyBlock (finalProgress=%.4f)", finalProgress)
					continue
				}

				p.blockBreakProgress = 0.0
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
					p.Dbg.Notify(DebugModeBlockBreaking, true, "(PlayerActionStartBreak) started breaking block at %v", action.BlockPos)
				}

				currentBlockBreakPos := p.worldUpdater.BlockBreakPos()
				if currentBlockBreakPos == nil || *currentBlockBreakPos != action.BlockPos {
					p.Dbg.Notify(DebugModeBlockBreaking, true, "reset block break progress (currentBlockBreakPos=%v, action.BlockPos=%v)", currentBlockBreakPos, action.BlockPos)
					p.blockBreakProgress = 0.0
				}
				p.blockBreakProgress += 1.0 / math32.Max(p.getExpectedBlockBreakTime(action.BlockPos), 0.001)
				p.worldUpdater.SetBlockBreakPos(&action.BlockPos)
			case protocol.PlayerActionContinueDestroyBlock:
				if currentBreakPos := p.worldUpdater.BlockBreakPos(); !p.blockBreakInProgress || (currentBreakPos != nil && *currentBreakPos != action.BlockPos) {
					p.blockBreakProgress = 0.0
					p.Dbg.Notify(DebugModeBlockBreaking, true, "different block being broken - reset block break progress (currentBreakPos=%v, action.BlockPos=%v)", currentBreakPos, action.BlockPos)
				}
				p.blockBreakProgress += 1.0 / math32.Max(p.getExpectedBlockBreakTime(action.BlockPos), 0.001)
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
					continue
				}

				p.blockBreakProgress += 1.0 / math32.Max(p.getExpectedBlockBreakTime(*p.worldUpdater.BlockBreakPos()), 0.001)
				p.Dbg.Notify(DebugModeBlockBreaking, true, "(PlayerActionStopBreak) block break progress=%.4f", p.blockBreakProgress)
				if p.blockBreakProgress <= 0.999 {
					p.SendBlockUpdates([]protocol.BlockPos{*p.worldUpdater.BlockBreakPos()})
					pk.InputData.Unset(packet.InputFlagPerformItemInteraction)
					p.Popup("<red>Broke block too early!</red>")
					p.Dbg.Notify(DebugModeBlockBreaking, true, "cancelled PlayerActionStopBreak (blockBreakProgress=%.4f)", p.blockBreakProgress)
					continue
				}

				p.blockBreakProgress = 0.0
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
	}

	if !handledBlockBreak && isFullServerAuthBlockBreaking {
		if blockBreakPos := p.worldUpdater.BlockBreakPos(); blockBreakPos != nil && p.blockBreakInProgress {
			p.blockBreakProgress += 1.0 / math32.Max(p.getExpectedBlockBreakTime(*blockBreakPos), 0.001)
			p.Dbg.Notify(DebugModeBlockBreaking, true, "(afterTheFactHack) block break in progress (blockBreakProgress=%.4f)", p.blockBreakProgress)
		}
	}

	/* if p.Ready {
		p.World().CleanChunks(p.worldUpdater.ChunkRadius(), chunkPos)
	} */
}

func (p *Player) getExpectedBlockBreakTime(pos protocol.BlockPos) float32 {
	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return 0
	}

	held, _ := p.HeldItems()
	if _, isShear := held.Item().(item.Shears); isShear {
		// FFS???
		return 1
	}

	// FIXME: It seems like Dragonfly doesn't have the item runtime IDs for Netherite tools set properly. This is a temporary
	// hack to allow netherite tools to work. However, it introduces a bypass where any nethite tool will be able to break
	// blocks instantly.
	if strings.Contains(utils.ItemName(held.Item()), "netherite") {
		return 0
	}

	b := p.World().Block(df_cube.Pos{int(pos.X()), int(pos.Y()), int(pos.Z())})
	if blockHash, _ := b.Hash(); blockHash == math.MaxUint64 {
		// If the block hash is MaxUint64, then the block is unknown to dragonfly. In the future,
		// we should implement more blocks to avoid this condition allowing clients to break those
		// blocks at any interval they please.
		return 0
	}

	if _, isAir := b.(block.Air); isAir {
		// Is it possible that the server already thinks the block is broken?
		return 0
	} else if utils.BlockName(b) == "minecraft:web" {
		// Cobwebs are not implemented in Dragonfly, and therefore the break time duration won't be accurate.
		// Just return 1 and accept when the client does break the cobweb.
		return 1
	} else if utils.BlockName(b) == "minecraft:bed" {
		return 1
	}

	breakTime := float32(block.BreakDuration(b, held).Milliseconds())
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
