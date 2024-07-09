package handler

import (
	"strings"

	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type DebugHandler struct {
	player.NopHandler
}

func NewDebugHandler() *DebugHandler {
	return &DebugHandler{}
}

func (DebugHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if t, ok := pk.(*packet.Text); ok {
		split := strings.Split(t.Message, " ")
		if split[0] != "!oomph_debug" {
			return true
		}

		if len(split) != 2 {
			p.Message("Usage: !oomph_debug <mode>")
			return false
		}

		var mode int
		switch split[1] {
		case "type:log":
			p.Dbg.LoggingType = player.LoggingTypeLogFile
			p.Message("Set debug logging type to <green>log file</green>.")
			return false
		case "type:message":
			p.Dbg.LoggingType = player.LoggingTypeMessage
			p.Message("Set debug logging type to <green>message</green>.")
			return false
		case "acks":
			mode = player.DebugModeACKs
		case "rotations":
			mode = player.DebugModeRotations
		case "combat":
			mode = player.DebugModeCombat
		case "clicks":
			mode = player.DebugModeClicks
		case "movement_sim":
			mode = player.DebugModeMovementSim
		case "latency":
			mode = player.DebugModeLatency
		case "chunks":
			mode = player.DebugModeChunks
		default:
			p.Message("Unknown debug mode: %s", split[1])
			return false
		}
		p.Dbg.Toggle(mode)
		if p.Dbg.Enabled(mode) {
			p.Message("<green>Enabled</green> debug mode: %s", split[1])
		} else {
			p.Message("<red>Disabled</red> debug mode: %s", split[1])
		}

		return false
	}

	return true
}
