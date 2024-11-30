package event

type MitigationEvent struct {
	Type      string  `json:"type"`
	SubType   string  `json:"sub_type"`
	ExtraData string  `json:"extra_data"`
	Count     float64 `json:"count"`
}

func (e *MitigationEvent) ID() string {
	return EventIDMitigation
}

func NewMitigationEvent(mitigationType, subType, extraData string, count float64) *MitigationEvent {
	return &MitigationEvent{
		Type:      mitigationType,
		SubType:   subType,
		ExtraData: extraData,
		Count:     count,
	}
}
