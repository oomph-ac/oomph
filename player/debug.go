package player

import "github.com/sirupsen/logrus"

const (
	DebugModeACKs = iota
	DebugModeRotations
	DebugModeCombat
	DebugModeClicks
	DebugModeMovementSim
	DebugModeLatency
	DebugModeChunks
	DebugModeAimA

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
	"movement_sim",
	"latency",
	"chunks",
	"aim-a",
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
	t.Log().SetLevel(logrus.DebugLevel)

	return d
}

// Toggle toggles the debug mode on/off based on the current state.
func (d *Debugger) Toggle(mode int) {
	if mode >= debugModeCount || mode < 0 {
		return
	}
	d.Modes[mode] = !d.Modes[mode]
}

// Enabled returns wether the debug mode is enabled or not.
func (d *Debugger) Enabled(mode int) bool {
	if mode >= debugModeCount || mode < 0 {
		return false
	}
	return d.Modes[mode]
}

func (d *Debugger) Notify(mode int, cond bool, msg string, args ...interface{}) {
	if !cond {
		return
	}

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
