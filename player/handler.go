package player

import (
	"github.com/df-mc/dragonfly/server/event"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Handler is an interface that can be implemented to have the player handle certain packets and events.
type Handler interface {
	// ID returns a string that identifies the handler.
	ID() string

	// HandleClientPacket handles a packet sent by a Minecraft client. It returns true
	// if the packet handled should be sent to the server and false if it should be dropped.
	HandleClientPacket(pk packet.Packet, p *Player) bool
	// HandleServerPacket handles a packet sent by a Minecraft server. It returns true
	// if the packet handled should be sent to the client and false if it should be dropped.
	HandleServerPacket(pk packet.Packet, p *Player) bool
	// OnTick is called every server tick.
	OnTick(p *Player)
}

// EventHandler is an interface that can be implemented to have the player handle certain events.
type EventHandler interface {
	// OnPunishment is called when a detection triggers a punishment for a player.
	OnPunishment(ctx *event.Context, p *Player, message *string)
	// OnFlagged is called when a detection flags a player.
	OnFlagged(ctx *event.Context, p *Player, detection Handler, data *orderedmap.OrderedMap[string, any])
}

// NopEventHandler is an event handler that does nothing.
type NopEventHandler struct {
}

func (NopEventHandler) OnPunishment(ctx *event.Context, p *Player, message *string) {
}

func (NopEventHandler) OnFlagged(ctx *event.Context, p *Player, detection Handler, data *orderedmap.OrderedMap[string, any]) {
}
