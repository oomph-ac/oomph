package event

const (
	AckEventSend = iota
	AckEventRefresh
)

type AckEvent struct {
	NopEvent

	SendTimestamp      int64
	RefreshedTimestmap int64
}

func (AckEvent) ID() byte {
	return EventIDAck
}

func (ev AckEvent) Encode() []byte {
	return DefaultEncode(ev)
}
