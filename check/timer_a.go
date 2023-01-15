package check

import (
	"math"
	"time"

	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TimerA checks for the timer cheat by using a balance system.
type TimerA struct {
	balance    float64
	lastTime   time.Time
	initalized bool
	basic
}

// NewTimerA creates a new TimerA check.
func NewTimerA() *TimerA {
	return &TimerA{}
}

// Name ...
func (*TimerA) Name() (string, string) {
	return "Timer", "A"
}

// Description ...
func (*TimerA) Description() string {
	return "This checks if a player is simulating ahead of the server."
}

// MaxViolations ...
func (*TimerA) MaxViolations() float64 {
	return math.MaxFloat64
}

// Process ...
func (t *TimerA) Process(p Processor, pk packet.Packet) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	curr := time.Now()
	// Get how many milliseconds have passed since the last input packet.
	timeDiff := float64(time.Since(t.lastTime).Microseconds()) / 1000

	defer func() {
		t.lastTime = curr
	}()

	if p.Respawned() || !p.Ready() {
		t.balance = 0
		return false
	}

	if !t.initalized {
		t.initalized = true
		return false
	}

	// The time difference should be one tick (50ms), so we subtract 50ms from the time difference and then add it to the balance.
	t.balance += timeDiff - 50
	if t.balance <= -150 {
		p.Flag(t, 1, map[string]any{"Balance": game.Round64(t.balance, 2)})
		t.balance = 0
	}

	return false
}
