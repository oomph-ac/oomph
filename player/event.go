package player

var EventIds = []string{
	"oomph:flagged",
	"oomph:latency_report",
	"oomph:authentication", // In gophertunnel fork.
}

type RemoteEvent interface {
	ID() string
}

type FlaggedEvent struct {
	Player     string  `json:"player"`
	Detection  string  `json:"check_main"`
	Type       string  `json:"check_sub"`
	Violations float32 `json:"violations"`
	ExtraData  string  `json:"extraData"`
}

func (e *FlaggedEvent) ID() string {
	return "oomph:flagged"
}

func NewFlaggedEvent(p *Player, t, st string, violations float32, extraData string) *FlaggedEvent {
	return &FlaggedEvent{
		Player:     p.Conn().IdentityData().DisplayName,
		Detection:  t,
		Type:       st,
		Violations: violations,
		ExtraData:  extraData,
	}
}

type UpdateLatencyEvent struct {
	RaknetLatency int64 `json:"raknet"`
	OomphLatency  int64 `json:"oomph"`
}

func (e *UpdateLatencyEvent) ID() string {
	return "oomph:latency_report"
}

func NewUpdateLatencyEvent(raknet, oomph int64) *UpdateLatencyEvent {
	return &UpdateLatencyEvent{
		RaknetLatency: raknet,
		OomphLatency:  oomph,
	}
}
