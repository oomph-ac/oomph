package event

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

func NewFlaggedEvent(player, t, st string, violations float32, extraData string) *FlaggedEvent {
	return &FlaggedEvent{
		Player:     player,
		Detection:  t,
		Type:       st,
		Violations: violations,
		ExtraData:  extraData,
	}
}
