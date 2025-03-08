package player

import (
	"strings"

	"github.com/df-mc/dragonfly/server/item"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

var DecodeClientPackets = []uint32{
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
}

func (p *Player) HandleClientPacket(pk packet.Packet) bool {
	defer p.recoverError()

	p.procMu.Lock()
	defer p.procMu.Unlock()

	cancel := false
	switch pk := pk.(type) {
	case *packet.ScriptMessage:
		if strings.Contains(pk.Identifier, "oomph:") {
			p.Disconnect("\t")
			return true
		}
	case *packet.Text:
		args := strings.Split(pk.Message, " ")
		if args[0] == "!oomph_debug" {
			if len(args) < 2 {
				p.Message("Usage: !oomph_debug <mode>")
				return true
			}

			var mode int
			switch args[1] {
			case "type:log":
				p.Log().SetLevel(logrus.DebugLevel)
				p.Dbg.LoggingType = LoggingTypeLogFile
				p.Message("Set debug logging type to <green>log file</green>.")
				return true
			case "type:message":
				p.Log().SetLevel(logrus.InfoLevel)
				p.Dbg.LoggingType = LoggingTypeMessage
				p.Message("Set debug logging type to <green>message</green>.")
				return true
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
			default:
				p.Message("Unknown debug mode: %s", args[1])
				return true
			}

			p.Dbg.Toggle(mode)
			if p.Dbg.Enabled(mode) {
				p.Message("<green>Enabled</green> debug mode: %s", args[1])
			} else {
				p.Message("<red>Disabled</red> debug mode: %s", args[1])
			}
			return true
		}
	case *packet.PlayerAuthInput:
		if !p.movement.InputAcceptable() {
			p.Popup("<red>input rate-limited (%d)</red>", p.SimulationFrame)
			return false
		}

		<-p.world.Exec(func(tx *df_world.Tx) {
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

			serverVerifiedHit := p.combat.Calculate()
			if serverVerifiedHit && missedSwing {
				pk.InputData.Unset(packet.InputFlagMissedSwing)
			}
		})
	case *packet.NetworkStackLatency:
		if p.ACKs().Execute(pk.Timestamp) {
			return true
		}
	case *packet.RequestChunkRadius:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.InventoryTransaction:
		earlyCancel := false
		<-p.world.Exec(func(tx *df_world.Tx) {
			p.worldTx = tx
			if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
				p.combat.Attack(pk)
				cancel = true
			} else if tr, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok && tr.ActionType == protocol.UseItemActionClickAir && p.Movement().Gliding() {
				p.inventory.SetHeldSlot(int32(tr.HotBarSlot))

				// If the client is gliding and uses a firework, it predicts a boost on it's own side, although the entity may not exist on the server.
				// This is very stange, as the gliding boost (in bedrock) is supplied by FireworksRocketActor::normalTick() which is similar to MC:JE logic.
				if _, isFireworks := p.inventory.Holding().Item().(item.Firework); isFireworks {
					p.movement.SetGlideBoost(20)
					p.Dbg.Notify(DebugModeMovementSim, true, "predicted client-sided glide booster for %d ticks", 20)
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
				p.SyncWorld()
				return true
			} */
			if !p.worldUpdater.AttemptBlockPlacement(pk) {
				earlyCancel = true
			}
		})

		if earlyCancel {
			return false
		}
	case *packet.MobEquipment:
		p.LastEquipmentData = pk
		p.inventory.SetHeldSlot(int32(pk.HotBarSlot))
	case *packet.Animate:
		if pk.ActionType == packet.AnimateActionSwingArm {
			p.Combat().Swing()
		}
	case *packet.ItemStackRequest:
		p.inventory.HandleItemStackRequest(pk)
	}

	p.RunDetections(pk)
	return cancel
}

func (p *Player) HandleServerPacket(pk packet.Packet) {
	defer p.recoverError()

	p.procMu.Lock()
	defer p.procMu.Unlock()

	switch pk := pk.(type) {
	case *packet.AddActor:
		width, height, scale := calculateBBSize(pk.EntityMetadata, 0.6, 1.8, 1.0)
		p.entTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			pk.EntityType,
			pk.EntityMetadata,
			pk.Position,
			pk.Velocity,
			p.entTracker.MaxRewind(),
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
			p.entTracker.MaxRewind(),
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
		p.handleEffectsPacket(pk)
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMoveActorAbsolute(pk)
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.MovePlayer:
		pk.Tick = 0
		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMovePlayer(pk)
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.RemoveActor:
		p.entTracker.RemoveEntity(uint64(pk.EntityUniqueID))
	case *packet.SetActorData:
		pk.Tick = 0
		if pk.EntityRuntimeID != p.RuntimeId {
			if e := p.entTracker.FindEntity(pk.EntityRuntimeID); e != nil {
				e.Width, e.Height, e.Scale = calculateBBSize(pk.EntityMetadata, e.Width, e.Height, e.Scale)
			}
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.SetActorMotion:
		pk.Tick = 0
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
		if pk.EntityRuntimeID == p.RuntimeId {
			p.movement.ServerUpdate(pk)
		}
	case *packet.UpdateBlock:
		p.worldUpdater.HandleUpdateBlock(pk)
	}
}
