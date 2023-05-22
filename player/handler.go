package player

import (
	"github.com/df-mc/dragonfly/server/event"
	"github.com/oomph-ac/oomph/check"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Handler handles events that are called by a player. Implementations of Handler may be used to listen to
// specific events such as when a player chats or moves.
type Handler interface {
	// HandlePunishment handles when a player should receive a punishment. Oomph doesn't punish players by default so
	// this should be used if you want to punish players.
	HandlePunishment(ctx *event.Context, check check.Check, message *string)
	// HandleFlag handles when a player gets flagged for a check. Log is true if Oomph should log the flag to the terminal.
	HandleFlag(ctx *event.Context, check check.Check, params map[string]any, log *bool)
	// HandleDebug handles check debug messages. These are logged by default and ctx.Cancel should be used to cancel them.
	HandleDebug(ctx *event.Context, check check.Check, params map[string]any)
	// HandleClientPacket handles when a player sends a packet to oomph.
	HandleClientPacket(ctx *event.Context, packet packet.Packet)
	// HandleServerPacket handles when a player receives a packet from oomph.
	HandleServerPacket(ctx *event.Context, packet packet.Packet)
}

// NopHandler implements the Handler interface but does not execute any code when an event is called. The
// default handler of players is set to NopHandler.
// Users may embed NopHandler to avoid having to implement each method.
type NopHandler struct{}

// Compile time check to make sure NopHandler implements Handler.
var _ Handler = (*NopHandler)(nil)

// HandlePunishment ...
func (NopHandler) HandlePunishment(*event.Context, check.Check, *string) {}

// HandleFlag ...
func (NopHandler) HandleFlag(*event.Context, check.Check, map[string]any, *bool) {}

// HandleDebug ...
func (NopHandler) HandleDebug(*event.Context, check.Check, map[string]any) {}

// HandleClientPacket ...
func (NopHandler) HandleClientPacket(*event.Context, packet.Packet) {}

// HandleServerPacket ...
func (NopHandler) HandleServerPacket(*event.Context, packet.Packet) {}
