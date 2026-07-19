package oconfig

import "sync"

const (
	PunishmentTypeNone = "none"
	PunishmentTypeKick = "kick"
	PunishmentTypeBan  = "ban"
)

var (
	DefaultDtcOpts = Detection{
		MaxVl:      10.0,
		FlagMsg:    "{prefix} §e{player} §7flagged §d{detection_type}§7(§cx{detection_subtype}§7) §7[§cx{violations}§7]",
		Punishment: PunishmentTypeNone,
	}
	dtcMutex sync.RWMutex
)

type Detection struct {
	MaxVl      float32 `json:"max_violations" comment:"The maximum amount of violations that Oomph will allow before taking action for this detection."`
	FlagMsg    string  `json:"flag_message" comment:"The message that will be sent to authorized staff when a player fails this detection."`
	Punishment string  `json:"punishment_type" comment:"The type of punishment to be applied when a player reaches the maximum amount of violations for this detection."`
}

func DtcOpts(dtc string) Detection {
	dtcMutex.RLock()
	defer dtcMutex.RUnlock()

	if dtc, ok := Global.Detections[dtc]; ok {
		return dtc
	}
	return DefaultDtcOpts
}

func ModifyDtcOpts(dtc string, opts Detection) {
	dtcMutex.Lock()
	defer dtcMutex.Unlock()

	Global.Detections[dtc] = opts
}
