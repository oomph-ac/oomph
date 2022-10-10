package check

import (
	"math"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type InvalidA struct {
	lastFrame uint64
	basic
}

func NewInvalidA() *InvalidA {
	return &InvalidA{}
}

// Name ...
func (*InvalidA) Name() (string, string) {
	return "Invalid", "A"
}

// Description ...
func (*InvalidA) Description() string {
	return "This checks if a player's simulation frame is valid."
}

// MaxViolations ...
func (*InvalidA) MaxViolations() float64 {
	return math.MaxFloat64
}

func (a *InvalidA) Process(p Processor, pk packet.Packet) bool {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return false
	}

	if a.lastFrame != 0 && i.Tick == 0 {
		p.Flag(a, 1, map[string]any{
			"Current":  i.Tick,
			"Previous": a.lastFrame,
		})
	}

	a.lastFrame = i.Tick
	return false
}
