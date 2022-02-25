package check

import "math"

// basic contains common fields utilized by all checks.
type basic struct {
	lastFlagTick uint64
	violations   float64
	buffer       float64
}

// Buff adds to the buffer and returns the new one.
func (t *basic) Buff(n float64, max ...float64) float64 {
	m := float64(15)
	if len(max) > 0 {
		m = max[0]
	}
	t.buffer += n
	t.buffer = math.Max(0, t.buffer)
	t.buffer = math.Min(t.buffer, m)
	return t.buffer
}

// TrackViolation ...
func (t *basic) TrackViolation() {
	t.violations++
}

// Violations ...
func (t *basic) Violations() float64 {
	return t.violations
}

// updateAndGetViolationAfterTicks ...
// TODO: what the fuck is this?
func (t *basic) updateAndGetViolationAfterTicks(tick uint64, maxTime float64) float64 {
	defer func() {
		t.lastFlagTick = tick
	}()
	return math.Max((maxTime+math.Min(float64(tick-t.lastFlagTick), 1))/maxTime, 0)
}
