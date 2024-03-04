package detection

import (
	"time"

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
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1

	d.lastTime = time.Now()
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
	timeDiff := float64(time.Since(d.lastTime).Microseconds()) / 1000

	defer func() {
		d.lastTime = curr
	}()

	if !p.Ready || !p.Alive {
		d.balance = 0
		return true
	}

	if !d.initalized {
		d.initalized = true
		return true
	}

	tickDelta := 1000 / float64(p.Tps)
	d.balance += timeDiff - tickDelta
	if d.balance <= -(tickDelta * 3) {
		d.Fail(p, nil)
		d.balance = 0
		return true
	}

	// This can occur if a user is attempting to use negative timer to increase their balance to a high amount,
	// to then use a high amount of timer after a period of time to bypass the check.
	if d.balance > 500 && p.ClientTick > p.ServerTick+5 && time.Since(p.LastServerTick).Milliseconds() < 100 {
		d.balance = 0
	}

	return true
}
