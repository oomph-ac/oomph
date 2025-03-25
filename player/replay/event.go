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

func EncodeEvent(e Event, w *protocol.Writer) {
	eventId := e.ID()
	w.Uint16(&eventId)
	e.Marshal(w)
}

func DecodeEvent(r *protocol.Reader) Event {
	var eventId uint16
	r.Uint16(&eventId)
	if f, ok := eventPool[eventId]; ok {
		e := f()
		e.Marshal(r)
		return e
	}
	return nil
}

func EventFromID(id uint16) Event {
	if f, ok := eventPool[id]; ok {
		return f()
	}
	return nil
}
