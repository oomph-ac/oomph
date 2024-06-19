package player

const (
	DebugModeACKs = iota
	DebugModeRotations
	DebugModeCombat
	DebugModeClicks

	debugModeCount
)

const (
	LoggingTypeMessage = iota
	LoggingTypeLogFile
)

var DebugModeList = []string{
	"acks",
	"rotations",
	"combat",
	"clicks",
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

func (d *Debugger) Notify(mode int, cond bool, msg string, args ...interface{}) {
	if v, ok := d.Modes[mode]; !ok || !v {
		return
	}

	switch d.LoggingType {
	case LoggingTypeLogFile:
		d.target.Log().Debugf("["+DebugModeList[mode]+"]: "+msg, args...)
	default:
		d.target.Message(msg, args...)
	}
}
