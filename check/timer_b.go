package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TimerB checks for the timer cheat by using a balance system.
type TimerB struct {
	balance  int64
	lastTime uint64
	basic
}

// NewTimerB creates a new TimerB check.
func NewTimerB() *TimerB {
	return &TimerB{}
}

// Name ...
func (*TimerB) Name() (string, string) {
	return "Timer", "B"
}

// Description ...
func (*TimerB) Description() string {
	return "This checks if a player is simulating inputs above 20 ticks per second."
}

// MaxViolations ...
func (*TimerB) MaxViolations() float64 {
	return 10
}

// Process ...
func (t *TimerB) Process(p Processor, pk packet.Packet) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if p.Dead() {
		t.balance = 0
		t.lastTime = 0
		return false
	}

	currentTime := p.ServerTick()
	if t.lastTime == 0 {
		t.lastTime = currentTime
		return false
	}

	// Get how many ticks have passed since the last input packet.
	timeDiff := currentTime - t.lastTime

	// The time difference should be one (tick), so we subtract one from the time difference and add it to the balance.
	t.balance += int64(timeDiff) - 1
	if t.balance == -5 {
		p.Flag(t, 1, map[string]any{"Balance": t.balance})
		t.balance = 0
	}
	t.lastTime = currentTime

	return false
}
