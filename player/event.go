package player

type RemoteEvent interface {
	ID() string
}

type FlaggedEvent struct {
	Player     string  `json:"player"`
	Detection  string  `json:"check_main"`
	Type       string  `json:"check_sub"`
	Violations float32 `json:"violations"`
}

func NewFlaggedEvent(p *Player, t, st string, violations float32) *FlaggedEvent {
	return &FlaggedEvent{
		Player:     p.Conn().IdentityData().DisplayName,
		Detection:  t,
		Type:       st,
		Violations: violations,
	}
}

func (e *FlaggedEvent) ID() string {
	return "oomph:flagged"
}
