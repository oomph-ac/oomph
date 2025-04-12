package player

import (
	"strings"

	"github.com/df-mc/dragonfly/server/item"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

var ClientDecode = []uint32{
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
}

func (p *Player) HandleClientPacket(ctx *context.HandlePacketContext) {
	defer p.recoverError()

	p.procMu.Lock()
	defer p.procMu.Unlock()

	p.pkCtx = ctx
	defer func() {
		p.pkCtx = nil
		p.worldTx = nil
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
		if args[0] == "!oomph_debug" {
			// If a player is running an oomph debug command, we don't want to leak that command into the chat.
			ctx.Cancel()
			if len(args) < 2 {
				p.Message("Usage: !oomph_debug <mode>")
				return
			}

			var mode int
			switch args[1] {
			case "type:log":
				p.Log().SetLevel(logrus.DebugLevel)
				p.Dbg.LoggingType = LoggingTypeLogFile
				p.Message("Set debug logging type to <green>log file</green>.")
				return
			case "type:message":
				p.Log().SetLevel(logrus.InfoLevel)
				p.Dbg.LoggingType = LoggingTypeMessage
				p.Message("Set debug logging type to <green>message</green>.")
				return
			case "acks":
				mode = DebugModeACKs
			case "rotations":
				mode = DebugModeRotations
			case "combat":
				mode = DebugModeCombat
			case "clicks":
				mode = DebugModeClicks
			case "movement":
				mode = DebugModeMovementSim
			case "latency":
				mode = DebugModeLatency
			case "chunks":
				mode = DebugModeChunks
			case "aim-a":
				mode = DebugModeAimA
			case "timer-a":
				mode = DebugModeTimerA
			case "block_placement":
				mode = DebugModeBlockPlacement
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
			ctx.Cancel()
			return
		}

		// Since Oomph utilizes a full-authoritative system for movement, we are always modifying the position in PlayerAuthInput packet
		// to Oomph's predicted position.
		ctx.SetModified()

		<-p.world.Exec(func(tx *df_world.Tx) {
			defer p.recoverError()

			p.worldTx = tx
			p.InputMode = pk.InputMode

			p.worldLoader.Move(tx, game.Vec32To64(p.Movement().Pos()))
			p.worldLoader.Load(tx, int(p.worldUpdater.ChunkRadius()))
			p.WorldUpdater().Tick()

			missedSwing := false
			if p.InputMode != packet.InputModeTouch && pk.InputData.Load(packet.InputFlagMissedSwing) {
				missedSwing = true
				p.combat.Attack(nil)
			}
			p.acks.Tick(true)

			p.handleBlockActions(pk)
			p.handleMovement(pk)

			if !oconfig.Combat().FullAuthoritative {
				p.entTracker.Tick(p.ClientTick)
			}

			serverVerifiedHit := p.combat.Calculate()
			if serverVerifiedHit && missedSwing {
				pk.InputData.Unset(packet.InputFlagMissedSwing)
			}
		})
	case *packet.NetworkStackLatency:
		if p.ACKs().Execute(pk.Timestamp) {
			ctx.Cancel()
			return
		}
	case *packet.RequestChunkRadius:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.InventoryTransaction:
		<-p.world.Exec(func(tx *df_world.Tx) {
			defer p.recoverError()

			p.worldTx = tx
			if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
				// The reason we cancel here is because Oomph also utlizes a full-authoritative system for combat. We need to wait for the
				// next movement (PlayerAuthInputPacket) the client sends so that we can accurately calculate if the hit is valid.
				p.combat.Attack(pk)
				ctx.Cancel()
				return
			} else if tr, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok {
				p.inventory.SetHeldSlot(int32(tr.HotBarSlot))
				if tr.ActionType == protocol.UseItemActionClickAir {
					// If the client is gliding and uses a firework, it predicts a boost on it's own side, although the entity may not exist on the server.
					// This is very stange, as the gliding boost (in bedrock) is supplied by FireworksRocketActor::normalTick() which is similar to MC:JE logic.
					held := p.inventory.Holding()
					if _, isFireworks := held.Item().(item.Firework); isFireworks && p.Movement().Gliding() {
						p.movement.SetGlideBoost(20)
						p.Dbg.Notify(DebugModeMovementSim, true, "predicted client-sided glide booster for %d ticks", 20)
					} else if utils.IsItemProjectile(held.Item()) {
						delta := p.InputCount - p.lastUseProjectileTick
						if delta < 4 {
							ctx.Cancel()
							_ = p.inventory.SyncSlot(protocol.WindowIDInventory, int(tr.HotBarSlot))
							p.Popup("<red>Item cooldown</red>")
							return
						}
						p.lastUseProjectileTick = p.InputCount
						inv, _ := p.inventory.WindowFromWindowID(protocol.WindowIDInventory)
						inv.SetSlot(int(tr.HotBarSlot), held.Grow(-1))
					} else if c, ok := held.Item().(item.Consumable); ok {
						if p.startUseConsumableTick == 0 {
							p.startUseConsumableTick = p.InputCount
							p.consumedSlot = int(tr.HotBarSlot)
						} else {
							duration := p.InputCount - p.startUseConsumableTick
							if duration < (c.ConsumeDuration().Milliseconds() / 50) {
								ctx.Cancel()
								_ = p.inventory.SyncSlot(protocol.WindowIDInventory, int(tr.HotBarSlot))
								p.Popup("<red>Item consumption cooldown</red>")
								return
							}
							p.startUseConsumableTick = 0
							p.consumedSlot = 0
						}
					}
				} else if tr.ActionType == protocol.UseItemActionBreakBlock && (p.GameMode == packet.GameTypeAdventure || p.GameMode == packet.GameTypeSurvival) {
					ctx.Cancel()
					return
				}
			} else if tr, ok := pk.TransactionData.(*protocol.ReleaseItemTransactionData); ok {
				p.inventory.SetHeldSlot(int32(tr.HotBarSlot))
			} else if _, ok := pk.TransactionData.(*protocol.NormalTransactionData); ok {
				if len(pk.Actions) != 2 {
					p.Log().Debugf("drop action should have exactly 2 actions, got %d", len(pk.Actions))
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
					p.Log().Debugf("missing information for drop action (foundItem=%v sourceSlot=%d droppedCount=%d)", foundClientItemStack, sourceSlot, droppedCount)
					return
				}

				inv, _ := p.inventory.WindowFromWindowID(protocol.WindowIDInventory)
				sourceSlotItem := inv.Slot(sourceSlot)
				if droppedCount > sourceSlotItem.Count() {
					p.Log().Debugf("dropped count (%d) is greater than source slot count (%d)", droppedCount, sourceSlotItem.Count())
					return
				}
				inv.SetSlot(sourceSlot, sourceSlotItem.Grow(-droppedCount))
			}

			/* if !p.worldUpdater.ValidateInteraction(pk) {
				ctx.Cancel()
			} */
			if !p.worldUpdater.AttemptBlockPlacement(pk) {
				ctx.Cancel()
			}
		})
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
		p.worldTx = nil
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
			oconfig.Combat().MaxRewind,
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
			oconfig.Combat().MaxRewind,
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
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.MovePlayer:
		pk.Tick = 0
		ctx.SetModified()

		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMovePlayer(pk)
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.RemoveActor:
		p.entTracker.RemoveEntity(uint64(pk.EntityUniqueID))
	case *packet.SetActorData:
		pk.Tick = 0
		ctx.SetModified()

		if pk.EntityRuntimeID != p.RuntimeId {
			if e := p.entTracker.FindEntity(pk.EntityRuntimeID); e != nil {
				e.Width, e.Height, e.Scale = calculateBBSize(pk.EntityMetadata, e.Width, e.Height, e.Scale)
			}
		} else {
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
	}
}
