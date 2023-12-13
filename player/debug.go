package player

import (
	"fmt"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	DebugTypeChat = iota
	DebugTypePopup
	DebugTypeLogged
)

type Debugger struct {
	LogLatency  bool
	LogCombat   bool
	LogMovement bool

	UseServerKnockback bool
	SmoothTeleports    bool

	AllowedDebug bool
}

func (p *Player) TryDebug(msg string, dtype int, log bool) {
	if !log {
		return
	}

	switch dtype {
	case DebugTypeChat:
		p.SendOomphDebug(msg, packet.TextTypeChat)
	case DebugTypePopup:
		p.SendOomphDebug(msg, packet.TextTypePopup)
	case DebugTypeLogged:
		p.Log().Debug(msg)
	default:
		panic(fmt.Errorf("unknown debug type %v", dtype))
	}
}
