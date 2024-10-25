package player

import (
	"fmt"
	"strings"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (p *Player) handleOneFromClient(pk packet.Packet) error {
	span := sentry.StartSpan(p.SentryTransaction.Context(), fmt.Sprintf("p.handleOneFromClient(%T)", pk))
	defer span.Finish()

	switch pk := pk.(type) {
	case *packet.ScriptMessage:
		if strings.Contains(pk.Identifier, "oomph:") {
			// TODO: Allow oomph to send messages to an API for logging instead of this hack to report to sentry.
			panic(oerror.New("malicious payload detected"))
		}
	case *packet.Text:
		split := strings.Split(pk.Message, " ")
		if split[0] != "!oomph_debug" {
			return nil
		}

		if len(split) != 2 {
			p.Message("Usage: !oomph_debug <mode>")
			return nil
		}

		var mode int
		switch split[1] {
		case "type:log":
			p.Log().SetLevel(logrus.DebugLevel)
			p.Dbg.LoggingType = LoggingTypeLogFile
			p.Message("Set debug logging type to <green>log file</green>.")
			return nil
		case "type:message":
			p.Log().SetLevel(logrus.InfoLevel)
			p.Dbg.LoggingType = LoggingTypeMessage
			p.Message("Set debug logging type to <green>message</green>.")
			return nil
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
			p.Message("Unknown debug mode: %s", split[1])
			return nil
		}

		p.Dbg.Toggle(mode)
		if p.Dbg.Enabled(mode) {
			p.Message("<green>Enabled</green> debug mode: %s", split[1])
		} else {
			p.Message("<red>Disabled</red> debug mode: %s", split[1])
		}
		return nil
	case *packet.PlayerAuthInput:
		p.handleBlockBreak(pk)
		p.handlePlayerMovementInput(pk)
	case *packet.NetworkStackLatency:
		if p.ACKs().Execute(pk.Timestamp) {
			return nil
		}
	case *packet.RequestChunkRadius:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.InventoryTransaction:
		if !p.worldUpdater.AttemptBlockPlacement(pk) {
			return nil
		}
	}

	cancel := false
	for _, h := range p.packetHandlers {
		cancel = cancel || !h.HandleClientPacket(pk, p)
	}

	det := p.RunDetections(pk)
	for _, h := range p.packetHandlers {
		h.Defer()
	}

	if !det || cancel {
		return nil
	}

	return p.SendPacketToServer(pk)
}

func (p *Player) handleOneFromServer(pk packet.Packet) error {
	span := sentry.StartSpan(p.SentryTransaction.Context(), fmt.Sprintf("p.handleOneFromServer(%T)", pk))
	defer span.Finish()

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
	case *packet.AddPlayer:
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
	case *packet.ChunkRadiusUpdated:
		p.worldUpdater.SetChunkRadius(pk.ChunkRadius + 4)
	case *packet.LevelChunk:
		p.worldUpdater.HandleLevelChunk(pk)
	case *packet.MobEffect:
		p.handleEffectsPacket(pk)
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId {
			p.entTracker.HandleMoveActorAbsolute(pk)
		} else {
			p.movement.ServerUpdate(pk)
		}
	case *packet.MovePlayer:
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

	cancel := false
	for _, h := range p.packetHandlers {
		cancel = cancel || !h.HandleServerPacket(pk, p)
	}

	if cancel {
		return nil
	}

	return p.SendPacketToClient(pk)
}
