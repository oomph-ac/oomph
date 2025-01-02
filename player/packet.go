package player

import (
	"strings"

	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/oerror"
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
}

func (p *Player) HandleClientPacket(pk packet.Packet) bool {
	p.procMu.Lock()
	defer p.procMu.Unlock()

	cancel := false
	switch pk := pk.(type) {
	case *packet.ScriptMessage:
		if strings.Contains(pk.Identifier, "oomph:") {
			// TODO: Allow oomph to send messages to an API for logging instead of this hack to report to sentry.
			panic(oerror.New("malicious payload detected"))
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
		p.InputMode = pk.InputMode

		missedSwing := false
		if p.InputMode != packet.InputModeTouch && pk.InputData.Load(packet.InputFlagMissedSwing) {
			missedSwing = true
			p.combat.Attack(nil)
		}

		p.clientEntTracker.Tick(p.ClientTick)
		p.handleBlockBreak(pk)
		p.handlePlayerMovementInput(pk)

		serverVerifiedHit := p.combat.Calculate()
		if serverVerifiedHit && missedSwing {
			pk.InputData.Unset(packet.InputFlagMissedSwing)
		}
		p.clientCombat.Calculate()
	case *packet.NetworkStackLatency:
		if p.ACKs().Execute(pk.Timestamp) {
			return false
		}
	case *packet.RequestChunkRadius:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			p.combat.Attack(pk)
			p.clientCombat.Attack(pk)
			cancel = true
		}

		if !p.worldUpdater.AttemptBlockPlacement(pk) {
			return true
		}
	case *packet.MobEquipment:
		p.LastEquipmentData = pk
	case *packet.Animate:
		if pk.ActionType == packet.AnimateActionSwingArm {
			p.ClientCombat().Swing()
		}
	}

	p.RunDetections(pk)
	return cancel
}

func (p *Player) HandleServerPacket(pk packet.Packet) {
	p.procMu.Lock()
	defer p.procMu.Unlock()

	switch pk := pk.(type) {
	case *packet.AddActor:
		width, height, scale := calculateBBSize(pk.EntityMetadata, 0.6, 1.8, 1.0)
		p.entTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			pk.Position,
			pk.Velocity,
			p.entTracker.MaxRewind(),
			false,
			width,
			height,
			scale,
		))
		p.clientEntTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			pk.Position,
			pk.Velocity,
			p.clientEntTracker.MaxRewind(),
			false,
			width,
			height,
			scale,
		))
	case *packet.AddPlayer:
		width, height, scale := calculateBBSize(pk.EntityMetadata, 0.6, 1.8, 1.0)
		p.entTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			pk.Position,
			pk.Velocity,
			p.entTracker.MaxRewind(),
			true,
			width,
			height,
			scale,
		))
		p.clientEntTracker.AddEntity(pk.EntityRuntimeID, entity.New(
			pk.Position,
			pk.Velocity,
			p.clientEntTracker.MaxRewind(),
			true,
			width,
			height,
			scale,
		))
	case *packet.ChunkRadiusUpdated:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.LevelChunk:
		p.worldUpdater.HandleLevelChunk(pk)
	case *packet.MobEffect:
		pk.Tick = 0
		p.handleEffectsPacket(pk)
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMoveActorAbsolute(pk)
			p.clientEntTracker.HandleMoveActorAbsolute(pk)
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.MovePlayer:
		pk.Tick = 0
		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMovePlayer(pk)
			p.clientEntTracker.HandleMovePlayer(pk)
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.RemoveActor:
		p.entTracker.RemoveEntity(uint64(pk.EntityUniqueID))
		p.clientEntTracker.RemoveEntity(uint64(pk.EntityUniqueID))
	case *packet.SetActorData:
		pk.Tick = 0
		if pk.EntityRuntimeID != p.RuntimeId {
			if e := p.entTracker.FindEntity(pk.EntityRuntimeID); e != nil {
				e.Width, e.Height, e.Scale = calculateBBSize(pk.EntityMetadata, e.Width, e.Height, e.Scale)
			}
			if e := p.clientEntTracker.FindEntity(pk.EntityRuntimeID); e != nil {
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
		pos := cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}
		b, ok := df_world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			p.Log().Errorf("unable to find block with runtime ID %v", pk.NewBlockRuntimeID)
			b = block.Air{}
		}

		p.World.SetBlock(df_cube.Pos(pos), b, nil)
	}
}
