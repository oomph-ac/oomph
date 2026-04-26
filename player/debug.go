package player

import "fmt"

const (
	DebugModeACKs = iota
	DebugModeRotations
	DebugModeCombat
	DebugModeMovementSim
	DebugModeLatency
	DebugModeChunks
	DebugModeAimA
	DebugModeTimer
	DebugModeBlockPlacement
	DebugModeUnhandledPackets
	DebugModeBlockBreaking
	DebugModeCrafting
	DebugModeItemRequests
	DebugModeBlockInteraction

	debugModeCount
)

const (
	LoggingTypeMessage = iota
	LoggingTypeLogFile
)

var (
	DebugModeList = []string{
		"acks",
		"rotations",
		"combat",
		"movement_sim",
		"latency",
		"chunks",
		"aim-a",
		"timer-a",
		"block_placements",
		"unhandled_packets",
		"block_breaking",
		"crafting",
		"item_requests",
		"block_interaction",
	}
	DebugModeMap = map[string]int{
		"acks":              DebugModeACKs,
		"rotations":         DebugModeRotations,
		"combat":            DebugModeCombat,
		"movement_sim":      DebugModeMovementSim,
		"latency":           DebugModeLatency,
		"chunks":            DebugModeChunks,
		"aim-a":             DebugModeAimA,
		"timer-a":           DebugModeTimer,
		"block_placements":  DebugModeBlockPlacement,
		"unhandled_packets": DebugModeUnhandledPackets,
		"block_breaking":    DebugModeBlockBreaking,
		"crafting":          DebugModeCrafting,
		"item_requests":     DebugModeItemRequests,
		"block_interaction": DebugModeBlockInteraction,
	}
)

type Debugger struct {
	Modes       map[int]bool
	LoggingType byte

	movSimBuf       []string
	bufferingMovSim bool

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
	if mode >= debugModeCount || mode < 0 {
		return
	}
	d.Modes[mode] = !d.Modes[mode]
}

// Enabled returns whether the debug mode is enabled or not.
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

	formatted := fmt.Sprintf(msg, args...)
	if mode == DebugModeMovementSim && d.bufferingMovSim {
		d.movSimBuf = append(d.movSimBuf, formatted)
		return
	}

	d.writeDebug(mode, formatted)
}

func (d *Debugger) writeDebug(mode int, msg string) {
	switch d.LoggingType {
	case LoggingTypeLogFile:
		d.target.Log().Debug("[" + DebugModeList[mode] + "]: " + msg)
	default:
		d.target.Message("[" + DebugModeList[mode] + "]: " + msg)
	}
}

// StartMovementSimBuffer begins buffering movement sim debug logs instead of writing them immediately.
func (d *Debugger) StartMovementSimBuffer() {
	d.movSimBuf = d.movSimBuf[:0]
	d.bufferingMovSim = true
}

// FlushMovementSimBuffer writes all buffered movement sim logs and ends buffering.
func (d *Debugger) FlushMovementSimBuffer() {
	for _, entry := range d.movSimBuf {
		d.writeDebug(DebugModeMovementSim, entry)
	}
	d.movSimBuf = d.movSimBuf[:0]
	d.bufferingMovSim = false
}

// DiscardMovementSimBuffer drops all buffered movement sim logs and ends buffering.
func (d *Debugger) DiscardMovementSimBuffer() {
	d.movSimBuf = d.movSimBuf[:0]
	d.bufferingMovSim = false
}
