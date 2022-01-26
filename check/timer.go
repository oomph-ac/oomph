package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TimerA checks for the timer cheat by using a balance system.
type TimerA struct {
	check
	balance  int64
	lastTime uint64
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
func (*TimerA) MaxViolations() uint32 {
	return 10
}

// Punishment ...
func (*TimerA) Punishment() punishment.Punishment {
	return punishment.Ban()
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
