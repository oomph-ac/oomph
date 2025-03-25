package replay

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	eventPool[IDTickEvent] = func() Event { return &TickEvent{} }
}

// TickEvent is an event called when the player's "server" goroutine ticks. There is no
// information needed to be encoded for this event.
type TickEvent struct{}

func (ev *TickEvent) ID() uint16 {
	return IDTickEvent
}

func (ev *TickEvent) Marshal(io protocol.IO) {}
