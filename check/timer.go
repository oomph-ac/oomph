package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TimerA checks for the timer cheat by using a balance system.
type TimerA struct {
	balance    int64
	lastTime   uint64
	violations uint32
}

// Name ...
func (*TimerA) Name() (string, string) {
	return "Timer", "A"
}

// Description ...
func (*TimerA) Description() string {
	return "Uses a 'balance' to determine if a player is using timer"
}

// Punishment ...
func (*TimerA) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Track ...
func (t *TimerA) Track() (uint32, uint32) {
	t.violations++
	return t.Violations()
}

// Violations ...
func (t *TimerA) Violations() (uint32, uint32) {
	return t.violations, 5
}

// Process ...
func (t *TimerA) Process(processor Processor, pk packet.Packet) {
	if _, ok := pk.(*packet.PlayerAuthInput); ok {
		currentTime := processor.Tick()
		if t.lastTime == 0 {
			t.lastTime = currentTime
			return
		}

		timeDiff := currentTime - t.lastTime
		t.balance += int64(timeDiff) - 1
		if t.balance <= -5 {
			processor.Flag(t, map[string]interface{}{"balance": t.balance})
			t.balance = 0
		}
		processor.Debug(t, map[string]interface{}{"balance": t.balance})
		t.lastTime = currentTime
	}
}
