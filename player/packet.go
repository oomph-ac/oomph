package player

import (
	"os"
	"strings"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var ClientDecode = []uint32{
	packet.IDInteract,
	packet.IDScriptMessage,
	packet.IDText,
	packet.IDPlayerAuthInput,
	packet.IDNetworkStackLatency,
	packet.IDRequestChunkRadius,
	packet.IDInventoryTransaction,
	packet.IDMobEquipment,
	packet.IDAnimate,
	packet.IDMovePlayer,
	packet.IDItemStackRequest,
	packet.IDLevelSoundEvent,
	packet.IDClientMovementPredictionSync,
	packet.IDPlayerAction,
}

var ServerDecode = []uint32{
	packet.IDAddActor,
	packet.IDAddPlayer,
	packet.IDChunkRadiusUpdated,
	packet.IDInventorySlot,
	packet.IDInventoryContent,
	packet.IDItemStackResponse,
	packet.IDLevelChunk,
	packet.IDMobEffect,
	packet.IDMoveActorAbsolute,
	packet.IDMovePlayer,
	packet.IDRemoveActor,
	packet.IDSetActorData,
	packet.IDSetActorMotion,
	packet.IDSetPlayerGameType,
	packet.IDSubChunk,
	packet.IDUpdateAbilities,
	packet.IDUpdateAttributes,
	packet.IDUpdateBlock,
	packet.IDUpdateSubChunkBlocks,
	packet.IDContainerOpen,
	packet.IDContainerClose,
	packet.IDCraftingData,
	packet.IDCreativeContent,
}

func (p *Player) HandleClientPacket(ctx *context.HandlePacketContext) {
	defer p.recoverError()

	p.procMu.Lock()
	defer p.procMu.Unlock()

	p.pkCtx = ctx
	defer func() {
		p.pkCtx = nil
	}()

	pk := *(ctx.Packet())
	switch pk := pk.(type) {
	case *packet.ScriptMessage:
		// TODO: Implement a better way to send messages to remote servers w/o abuse of ScriptMessagePacket.
		if strings.Contains(pk.Identifier, "oomph:") {
			p.Disconnect("\t")
			return
		}
	case *packet.Text:
		args := strings.Split(pk.Message, " ")
		if args[0] == "!oomph_debug" && p.Opts().UseDebugCommands {
			// If a player is running an oomph debug command, we don't want to leak that command into the chat.
			ctx.Cancel()
			if len(args) < 2 {
				p.Message("Usage: !oomph_debug <mode>")
				return
			}

			var mode int
			switch args[1] {
			case "gmc":
				if len(os.Getenv("GMC_TEST_BECAUSE_DEVLOL")) > 0 {
					p.SendPacketToClient(&packet.SetPlayerGameType{
						GameType: packet.GameTypeCreative,
					})
				} else {
					p.Message("hi there :3c")
				}
				return
			case "type:log":
				p.Dbg.LoggingType = LoggingTypeLogFile
				p.Message("Set debug logging type to <green>log file</green>.")
				return
			case "type:message":
				p.Dbg.LoggingType = LoggingTypeMessage
				p.Message("Set debug logging type to <green>message</green>.")
				return
			case "acks":
				mode = DebugModeACKs
			case "rotations":
				mode = DebugModeRotations
			case "combat":
				mode = DebugModeCombat
			case "movement":
				mode = DebugModeMovementSim
			case "latency":
				mode = DebugModeLatency
			case "chunks":
				mode = DebugModeChunks
			case "aim-a":
				mode = DebugModeAimA
			case "timer":
				mode = DebugModeTimer
			case "block_placement":
				mode = DebugModeBlockPlacement
			case "unhandled_packets":
				mode = DebugModeUnhandledPackets
			case "block_breaking", "block_break":
				mode = DebugModeBlockBreaking
			case "block_interaction":
				mode = DebugModeBlockInteraction
			default:
				p.Message("Unknown debug mode: %s", args[1])
				return
			}

			p.Dbg.Toggle(mode)
			if p.Dbg.Enabled(mode) {
				p.Message("<green>Enabled</green> debug mode: %s", args[1])
			} else {
				p.Message("<red>Disabled</red> debug mode: %s", args[1])
			}
			return
		}
	case *packet.PlayerAuthInput:
		if !p.movement.InputAcceptable() {
			p.Popup("<red>input rate-limited (%d)</red>", p.SimulationFrame)
			p.tryRunningClientCombat()
			ctx.Cancel()
			return
		}

		// Since Oomph utilizes a full-authoritative system for movement, we are always modifying the position in PlayerAuthInput packet
		// to Oomph's predicted position.
		ctx.SetModified()

		p.InputMode = pk.InputMode
		missedSwing := false
		if p.InputMode != packet.InputModeTouch && pk.InputData.Load(packet.InputFlagMissedSwing) {
			missedSwing = true
			p.combat.Attack(nil)
		}
		p.acks.Tick(true)

		if pk.InputData.Load(packet.InputFlagPerformItemStackRequest) {
			p.inventory.HandleSingleRequest(pk.ItemStackRequest)
		}

		p.handleBlockActions(pk)
		p.handleMovement(pk)
		p.tryRunningClientCombat()

		var serverVerifiedHit bool
		if !p.blockBreakInProgress {
			// The client should not be able to hit any entities while breaking a block.
			serverVerifiedHit = p.combat.Calculate()
		} else {
			p.combat.Reset()
		}
		if serverVerifiedHit && missedSwing {
			pk.InputData.Unset(packet.InputFlagMissedSwing)
		}
	case *packet.NetworkStackLatency:
		if p.ACKs().Execute(pk.Timestamp) {
			ctx.Cancel()
			return
		}
	case *packet.RequestChunkRadius:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			// The reason we cancel here is because Oomph also utlizes a full-authoritative system for combat. We need to wait for the
			// next movement (PlayerAuthInputPacket) the client sends so that we can accurately calculate if the hit is valid.
			p.combat.Attack(pk)
			if p.opts.Combat.EnableClientEntityTracking {
				p.clientCombat.Attack(pk)
			}
			ctx.Cancel()
			//return
		} else if tr, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok {
			p.inventory.SetHeldSlot(int32(tr.HotBarSlot))
			if tr.ActionType == protocol.UseItemActionClickAir {
				// If the client is gliding and uses a firework, it predicts a boost on it's own side, although the entity may not exist on the server.
				// This is very stange, as the gliding boost (in bedrock) is supplied by FireworksRocketActor::normalTick() which is similar to MC:JE logic.
				held := p.inventory.Holding()
				if _, isFireworks := held.Item().(item.Firework); isFireworks && p.Movement().Gliding() {
					p.movement.SetGlideBoost(game.GlideBoostTicks)
					p.Dbg.Notify(DebugModeMovementSim, true, "predicted client-sided glide booster for %d ticks", game.GlideBoostTicks)
				} else if utils.IsItemProjectile(held.Item()) {
					delta := p.InputCount - p.lastUseProjectileTick
					if delta < 4 {
						ctx.Cancel()
						p.inventory.ForceSync()
						p.Popup("<red>Projectile cooldown (%d)</red>", 4-delta)
						return
					}
					p.lastUseProjectileTick = p.InputCount
					inv, _ := p.inventory.WindowFromWindowID(protocol.WindowIDInventory)
					inv.SetSlot(int(tr.HotBarSlot), held.Grow(-1))
				} /* else if c, ok := held.Item().(item.Consumable); ok {
					if p.startUseConsumableTick == 0 {
						p.startUseConsumableTick = p.InputCount
						p.consumedSlot = int(tr.HotBarSlot)
					} else {
						duration := p.InputCount - p.startUseConsumableTick
						if duration < ((c.ConsumeDuration().Milliseconds() / 50) - 1) {
							p.startUseConsumableTick = p.InputCount
							ctx.Cancel()
							p.inventory.ForceSync()
							p.Message("item cooldown (attempted to consume in %d ticks, %d required)", duration, (c.ConsumeDuration().Milliseconds()/50)-1)
							//_ = p.inventory.SyncSlot(protocol.WindowIDInventory, int(tr.HotBarSlot))
							//p.Popup("<red>Item consumption cooldown</red>")
							return
						}
						p.startUseConsumableTick = 0
						p.consumedSlot = 0
					}
				} */
			}
		} else if tr, ok := pk.TransactionData.(*protocol.ReleaseItemTransactionData); ok {
			p.inventory.SetHeldSlot(int32(tr.HotBarSlot))
		} else if _, ok := pk.TransactionData.(*protocol.NormalTransactionData); ok {
			if len(pk.Actions) != 2 {
				p.Log().Debug("drop action should have exactly 2 actions, got different amount", "actionCount", len(pk.Actions))
				if len(pk.Actions) > 5 {
					p.Disconnect("Error: Too many actions in NormalTransactionData")
				}
				return
			}

			var (
				sourceSlot           int = -1
				droppedCount         int = -1
				foundClientItemStack bool
			)

			for _, action := range pk.Actions {
				if action.SourceType == protocol.InventoryActionSourceWorld && action.InventorySlot == 0 {
					droppedCount = int(action.NewItem.Stack.Count)
				} else if action.SourceType == protocol.InventoryActionSourceContainer && action.WindowID == protocol.WindowIDInventory {
					sourceSlot = int(action.InventorySlot)
					foundClientItemStack = true
				}
			}

			if !foundClientItemStack || sourceSlot == -1 || droppedCount == -1 {
				p.Log().Debug("missing information for drop action", "foundItem", foundClientItemStack, "srcSlot", sourceSlot, "dropCount", droppedCount)
				return
			}

			inv, _ := p.inventory.WindowFromWindowID(protocol.WindowIDInventory)
			sourceSlotItem := inv.Slot(sourceSlot)
			if droppedCount > sourceSlotItem.Count() {
				p.Log().Debug("dropped count is greater than source slot count", "droppedCount", droppedCount, "available", sourceSlotItem.Count())
				return
			}
			inv.SetSlot(sourceSlot, sourceSlotItem.Grow(-droppedCount))
		}

		interactionValid := p.worldUpdater.ValidateInteraction(pk)
		if !interactionValid {
			ctx.Cancel()
		} else if !p.worldUpdater.AttemptItemInteractionWithBlock(pk) {
			ctx.Cancel()
		}
	case *packet.MobEquipment:
		p.LastEquipmentData = pk
		p.inventory.SetHeldSlot(int32(pk.HotBarSlot))
		if p.startUseConsumableTick != 0 && p.consumedSlot != int(pk.HotBarSlot) {
			p.startUseConsumableTick = 0
			p.consumedSlot = 0
		}
	case *packet.Animate:
		if pk.ActionType == packet.AnimateActionSwingArm {
			p.Combat().Swing()
		}
	case *packet.ItemStackRequest:
		p.inventory.HandleItemStackRequest(pk)
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			p.Combat().Attack(nil)
		}
	}
	p.RunDetections(pk)
}

func (p *Player) HandleServerPacket(ctx *context.HandlePacketContext) {
	defer p.recoverError()

	p.procMu.Lock()
	defer p.procMu.Unlock()

	p.pkCtx = ctx
	defer func() {
		p.pkCtx = nil
	}()

	pk := *(ctx.Packet())
	switch pk := pk.(type) {
	case *packet.AddActor:
		width, height, scale := calculateBBSize(pk.EntityMetadata, 0.6, 1.8, 1.0)
		p.entTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			pk.EntityType,
			pk.EntityMetadata,
			pk.Position,
			pk.Velocity,
			p.Opts().Combat.MaxRewind,
			false,
			width,
			height,
			scale,
		))
		p.clientEntTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			pk.EntityType,
			pk.EntityMetadata,
			pk.Position,
			pk.Velocity,
			p.Opts().Combat.MaxRewind,
			false,
			width,
			height,
			scale,
		))
	case *packet.AddPlayer:
		width, height, scale := calculateBBSize(pk.EntityMetadata, 0.6, 1.8, 1.0)
		p.entTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			"",
			pk.EntityMetadata,
			pk.Position,
			pk.Velocity,
			p.Opts().Combat.MaxRewind,
			true,
			width,
			height,
			scale,
		))
		p.clientEntTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			"",
			pk.EntityMetadata,
			pk.Position,
			pk.Velocity,
			p.Opts().Combat.MaxRewind,
			true,
			width,
			height,
			scale,
		))
	case *packet.ChunkRadiusUpdated:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.InventorySlot:
		p.inventory.HandleInventorySlot(pk)
	case *packet.InventoryContent:
		p.inventory.HandleInventoryContent(pk)
	case *packet.ItemStackResponse:
		p.inventory.HandleItemStackResponse(pk)
	case *packet.LevelChunk:
		p.worldUpdater.HandleLevelChunk(pk)
	case *packet.MobEffect:
		pk.Tick = 0
		ctx.SetModified()
		p.Movement().ServerUpdate(pk)
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMoveActorAbsolute(pk)
			if p.opts.Combat.EnableClientEntityTracking {
				p.clientEntTracker.HandleMoveActorAbsolute(pk)
			}
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.MovePlayer:
		pk.Tick = 0
		ctx.SetModified()

		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMovePlayer(pk)
			if p.Opts().Combat.EnableClientEntityTracking {
				p.clientEntTracker.HandleMovePlayer(pk)
			}
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.RemoveActor:
		p.entTracker.RemoveEntity(uint64(pk.EntityUniqueID))
		p.clientEntTracker.RemoveEntity(uint64(pk.EntityUniqueID))
	case *packet.SetActorData:
		pk.Tick = 0
		ctx.SetModified()

		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleSetActorData(pk)
			p.clientEntTracker.HandleSetActorData(pk)
		} else {
			copyPk := *pk
			p.LastSetActorData = &copyPk
			p.movement.ServerUpdate(pk)
		}
	case *packet.SetActorMotion:
		pk.Tick = 0
		ctx.SetModified()

		if pk.EntityRuntimeID == p.RuntimeId {
			p.movement.ServerUpdate(pk)
		}
	case *packet.SetPlayerGameType:
		p.gamemodeHandle.Handle(pk)
	case *packet.SubChunk:
		p.worldUpdater.HandleSubChunk(pk)
	case *packet.UpdateAbilities:
		if pk.AbilityData.EntityUniqueID == p.UniqueId {
			p.movement.ServerUpdate(pk)
		}
	case *packet.UpdateAttributes:
		pk.Tick = 0
		ctx.SetModified()

		if pk.EntityRuntimeID == p.RuntimeId {
			p.movement.ServerUpdate(pk)
		}
	case *packet.UpdateBlock:
		p.worldUpdater.HandleUpdateBlock(pk)
	case *packet.UpdateSubChunkBlocks:
		p.worldUpdater.HandleUpdateSubChunkBlocks(pk)
	case *packet.ContainerOpen:
		p.inventory.CreateWindow(pk.WindowID, pk.ContainerType)
	case *packet.ContainerClose:
		p.inventory.RemoveWindow(pk.WindowID)
	case *packet.CraftingData:
		if pk.ClearRecipes {
			p.Recipies = make(map[uint32]protocol.Recipe)
		}
		for _, recp := range pk.Recipes {
			switch recp := recp.(type) {
			case *protocol.ShapedRecipe:
				p.Recipies[recp.RecipeNetworkID] = recp
			case *protocol.ShapelessRecipe:
				p.Recipies[recp.RecipeNetworkID] = recp
			case *protocol.MultiRecipe:
				p.Recipies[recp.RecipeNetworkID] = recp
			case *protocol.SmithingTransformRecipe:
				p.Recipies[recp.RecipeNetworkID] = recp
			case *protocol.SmithingTrimRecipe:
				p.Recipies[recp.RecipeNetworkID] = recp
			}
		}
	case *packet.CreativeContent:
		p.CreativeItems = make(map[uint32]protocol.CreativeItem)
		for _, item := range pk.Items {
			p.CreativeItems[item.CreativeItemNetworkID] = item
		}
	}
}
