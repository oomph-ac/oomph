package check

import (
	"math"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TimerA checks for the timer cheat by using a balance system.
type TimerA struct {
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

	// The check cannot be run at this point in time because the client has not recieved
	// a sync yet from the server.
	if !p.IsSyncedWithServer() || !p.ServerTickingStable() {
		return false
	}

	// Here, we check if the client is simulating ahead of the server.
	if p.ClientTick() > p.ServerTick() {
		p.Flag(t, 1, map[string]any{
			"Server Tick": p.ServerTick(),
			"Client Tick": p.ClientTick(),
		})
	}

	return false
}
