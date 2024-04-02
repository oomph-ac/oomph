package event

type TickEvent struct {
	NopEvent

	Tick int64
}

func (TickEvent) ID() byte {
	return EventIDServerTick
}

func (ev TickEvent) Encode() []byte {
	return DefaultEncode(ev)
}
