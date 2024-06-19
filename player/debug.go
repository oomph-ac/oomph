package player

const (
	DebugModeACKs = iota

	debugModeCount
)

const (
	LoggingTypeMessage = iota
	LoggingTypeLogFile
)

var DebugModeList = []string{
	"acks",
}

type Debugger struct {
	Modes       map[int]bool
	LoggingType byte

	target *Player
}

func NewDebugger(t *Player) *Debugger {
	d := &Debugger{
		Modes:       make(map[int]bool),
		LoggingType: LoggingTypeLogFile,

		target: t,
	}
	for mode := range DebugModeList {
		d.Modes[mode] = false
	}

	return d
}

// Toggle toggles the debug mode on/off based on the current state.
func (d *Debugger) Toggle(mode int) {
	d.Modes[mode] = !d.Modes[mode]
}

func (d *Debugger) Notify(mode int, msg string, args ...interface{}) {
	if !d.Modes[mode] {
		return
	}

	switch d.LoggingType {
	case LoggingTypeLogFile:
		d.target.Log().Infof(msg, args...)
	default:
		d.target.Message(msg, args...)
	}
}
