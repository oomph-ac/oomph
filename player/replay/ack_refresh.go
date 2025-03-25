package replay

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	eventPool[IDAckRefreshEvent] = func() Event { return &AckRefreshEvent{} }
}

// AckRefreshEvent is an event that is called when the ACK component of the player refreshes its timestamp.
type AckRefreshEvent struct {
	// Timestamp is the new timestamp of the ACK component that Oomph predicted. This is
	// neccessary for the replay system, as not sending it would lead in a mismatch
	// between the reaL-time ack component ID and the replay-time ack component ID.
	// (The mismatch would happen because the timestamp is randomly generated.)
	Timestamp int64
}

func (ev *AckRefreshEvent) ID() uint16 {
	return IDAckRefreshEvent
}

func (ev *AckRefreshEvent) Marshal(io protocol.IO) {
	io.Int64(&ev.Timestamp)
}
