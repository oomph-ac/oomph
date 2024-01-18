package detection

import (
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDTimerA = "oomph:timer_a"

type TimerA struct {
	BaseDetection

	balance    float64
	lastTime   time.Time
	initalized bool
}

func NewTimerA() *TimerA {
	d := &TimerA{}
	d.Type = "Timer"
	d.SubType = "A"

	d.Description = "Detects if a player is simulating ahead of the server"
	d.Punishable = true

	d.MaxViolations = 15
	d.trustDuration = 200 * player.TicksPerSecond

	d.FailBuffer = 1.5
	d.MaxBuffer = 4
	return d
}

func (d *TimerA) ID() string {
	return DetectionIDTimerA
}

func (d *TimerA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MovementMode != player.AuthorityModeSemi {
		return true
	}
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}
	
	curr := time.Now()
	// Get how many milliseconds have passed since the last input packet.
	timeDiff := float64(time.Since(d.lastTime).Microseconds()) / 1000

	defer func() {
		d.lastTime = curr
	}()

	if !p.Ready {
		d.balance = 0
		return true
	}

	if !d.initalized {
		d.initalized = true
		return true
	}

	d.balance += timeDiff - 50
	if d.balance <= -150 {
		dat := orderedmap.NewOrderedMap[string, any]()
		dat.Set("client_tick", p.ClientTick)
		dat.Set("server_tick", p.ServerTick)
		d.Fail(p, dat)
		d.balance = 0
		return false
	}

	// This can occur if a user is attempting to use negative timer to increase their balance to a high amount,
	// to then use a high amount of timer after a period of time to bypass the check.
	if d.balance > 500 && p.ClientTick > p.ServerTick {
		d.balance = 0
	}

	return true
}
