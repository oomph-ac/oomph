package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TimerA checks for the timer cheat by using a balance system.
type TimerA struct {
	check
	balance   int64
	lastTime  uint64
	clientTPS float64
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

		// get how many ticks have passed since the last input packet.
		timeDiff := currentTime - t.lastTime
		// timeDiff should be 1, so we subtract 1 from the timeDiff and add it to the balance.
		t.balance += int64(timeDiff) - 1
		if t.balance == -5 {
			processor.Flag(t, map[string]interface{}{"timer": omath.Round(t.clientTPS/float64(20), 4)})
			t.balance = 0
		}
		t.lastTime = currentTime
	}
}
