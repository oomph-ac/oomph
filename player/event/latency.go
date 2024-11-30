package event

type UpdateLatencyEvent struct {
	RaknetLatency int64 `json:"raknet"`
	OomphLatency  int64 `json:"oomph"`
}

func (e *UpdateLatencyEvent) ID() string {
	return EventIDLatencyReport
}

func NewUpdateLatencyEvent(raknet, oomph int64) *UpdateLatencyEvent {
	return &UpdateLatencyEvent{
		RaknetLatency: raknet,
		OomphLatency:  oomph,
	}
}
