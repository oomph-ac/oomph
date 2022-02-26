package check

import (
	"github.com/justtaldevelops/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TimerA checks for the timer cheat by using a balance system.
type TimerA struct {
	basic
	balance   int64
	lastTime  uint64
	clientTPS float64
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
	return "This checks if a player is sending movement packets too fast."
}

// MaxViolations ...
func (*TimerA) MaxViolations() float64 {
	return 10
}

// Process ...
func (t *TimerA) Process(processor Processor, pk packet.Packet) {
	if !processor.Ready() {
		// Wait until we're spawned in to prevent falses on join.
		return
	}

	if _, ok := pk.(*packet.PlayerAuthInput); ok {
		currentTime := processor.ServerTick()
		if t.lastTime == 0 {
			t.lastTime = currentTime
			return
		}
		if currentTime%20 == 0 {
			t.clientTPS = 0
		} else {
			t.clientTPS++
		}

		// Get how many ticks have passed since the last input packet.
		timeDiff := currentTime - t.lastTime

		// The time difference should be one, so we subtract one from the time difference and add it to the balance.
		t.balance += int64(timeDiff) - 1
		if t.balance == -5 {
			processor.Flag(t, 1, map[string]interface{}{
				"Timer": game.Round(t.clientTPS/float64(20), 4)},
			)
			t.balance = 0
		}
		t.lastTime = currentTime
	}
}
