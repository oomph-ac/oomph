package check

import "math"

// basic contains common fields utilized by all checks.
type basic struct {
	lastFlagTick uint64
	violations   float64
	buffer       float64
}

// Buff adds to the buffer and returns the new one.
func (b *basic) Buff(n float64, max ...float64) float64 {
	m := 15.0
	if len(max) > 0 {
		m = max[0]
	}
	b.buffer += n
	b.buffer = math.Max(0, b.buffer)
	b.buffer = math.Min(b.buffer, m)
	return b.buffer
}

// AddViolation...
func (b *basic) AddViolation(v float64) {
	b.violations += v
}

// Violations ...
func (b *basic) Violations() float64 {
	return b.violations
}

// violationAfterTicks ...
func (b *basic) violationAfterTicks(tick uint64, maxTicks uint64) float64 {
	diff := float64(tick - b.lastFlagTick)
	b.lastFlagTick = tick
	return math.Max(((float64(maxTicks)+math.Min(diff, 1))-diff)/float64(maxTicks), 0)
}
