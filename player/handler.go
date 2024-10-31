package player

import (
	"github.com/df-mc/dragonfly/server/event"
	"github.com/elliotchance/orderedmap/v2"
)

type EventHandler interface {
	// HandlePunishment is called when a detection triggers a punishment for a player.
	HandlePunishment(ctx *event.Context, detection Detection, message *string)
	// HandleFlag is called when a detection flags a player.
	HandleFlag(ctx *event.Context, detection Detection, data *orderedmap.OrderedMap[string, any])
	// HandleQuit is called when a player is closed.
	HandleQuit()
}

// NopEventHandler is an event handler that does nothing.
type NopEventHandler struct{}

func (NopEventHandler) HandlePunishment(*event.Context, Detection, *string) {}

func (NopEventHandler) HandleFlag(*event.Context, Detection, *orderedmap.OrderedMap[string, any]) {
}

func (NopEventHandler) HandleQuit() {}
