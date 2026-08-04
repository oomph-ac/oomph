package player

import (
	"fmt"

	"github.com/oomph-ac/oomph/anticheat/utils"
)

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

// movSimHistoryTicks is the maximum amount of ticks of movement sim debug logs that are retained.
const movSimHistoryTicks = 10

type Debugger struct {
	Modes       map[int]bool
	LoggingType byte

	movSimBuf []string
	// movSimHistory holds the logs of the last movSimHistoryTicks ticks. Slots that hold no
	// logs are nil.
	movSimHistory   *utils.CircularQueue[[]string]
	bufferingMovSim bool

	target *Player
}

func NewDebugger(t *Player) *Debugger {
	d := &Debugger{
		Modes:       make(map[int]bool),
		LoggingType: LoggingTypeLogFile,

		movSimHistory: utils.NewCircularQueue[[]string](movSimHistoryTicks, nil),

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
		d.target.Message("%s", "["+DebugModeList[mode]+"]: "+msg)
	}
}

// StartMovementSimBuffer begins buffering movement sim debug logs instead of writing them immediately.
func (d *Debugger) StartMovementSimBuffer() {
	d.movSimBuf = d.movSimBuf[:0]
	d.bufferingMovSim = true
}

// FlushMovementSimBuffer writes the retained movement sim logs of previous ticks, then the logs of
// the current tick, and ends buffering. The retained history is cleared afterwards.
func (d *Debugger) FlushMovementSimBuffer() {
	retained := 0
	for tickLogs := range d.movSimHistory.Iter() {
		if len(tickLogs) > 0 {
			retained++
		}
	}

	written := 0
	for tickLogs := range d.movSimHistory.Iter() {
		if len(tickLogs) == 0 {
			continue
		}
		d.writeDebug(DebugModeMovementSim, fmt.Sprintf("--- tick history (-%d) ---", retained-written))
		for _, entry := range tickLogs {
			d.writeDebug(DebugModeMovementSim, entry)
		}
		written++
	}
	if retained > 0 {
		d.writeDebug(DebugModeMovementSim, "--- current tick ---")
	}
	for _, entry := range d.movSimBuf {
		d.writeDebug(DebugModeMovementSim, entry)
	}

	// Clear the history so the flushed log strings can be garbage-collected.
	for i := 0; i < d.movSimHistory.Size(); i++ {
		_ = d.movSimHistory.Set(i, nil)
	}
	d.movSimBuf = d.movSimBuf[:0]
	d.bufferingMovSim = false
}

// DiscardMovementSimBuffer moves the buffered logs of the current tick into the retained history and
// ends buffering. The history holds the logs of at most the last movSimHistoryTicks ticks.
func (d *Debugger) DiscardMovementSimBuffer() {
	if len(d.movSimBuf) > 0 {
		// Append evicts the oldest slot, so its backing array can be recycled as the
		// buffer for the next tick.
		recycled, _ := d.movSimHistory.Get(0)
		_ = d.movSimHistory.Append(d.movSimBuf)
		d.movSimBuf = recycled[:0]
	}
	d.bufferingMovSim = false
}
