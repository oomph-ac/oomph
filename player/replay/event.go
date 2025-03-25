package replay

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	IDPacketEvent uint16 = iota
	IDTickEvent
	IDAckRefreshEvent
)

var eventPool = make(map[uint16]func() Event)

type Event interface {
	ID() uint16
	Marshal(io protocol.IO)
}

func EventFromID(id uint16) Event {
	if f, ok := eventPool[id]; ok {
		return f()
	}
	return nil
}
