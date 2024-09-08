package detection

import (
	"math"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	DetectionIDTimerA    = "oomph:timer_a"
	timerATickSampleSize = 10
)

type TimerA struct {
	BaseDetection

	tickTimes     []float64
	tickTimeCount uint8

	balance  float64
	lastTime time.Time
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

	d.tickTimes = make([]float64, timerATickSampleSize)

	d.lastTime = time.Now()
	d.balance = 0

	return d
}

func (d *TimerA) ID() string {
	return DetectionIDTimerA
}

func (d *TimerA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	curr := p.Time()
	timeDiff := float64(time.Since(d.lastTime).Microseconds()) / 1000
	d.tickTimes[d.tickTimeCount] = timeDiff
	d.tickTimeCount++

	defer func() {
		d.lastTime = curr

		if d.tickTimeCount == 10 {
			for i := 1; i < 10; i++ {
				d.tickTimes[i-1] = d.tickTimes[i]
			}
			d.tickTimeCount--
		}
	}()

	/* if !p.Alive {
		d.balance = 0
		return true
	} */

	tickDelta := 1000 / float64(p.Tps)
	d.balance += timeDiff - tickDelta
	if d.balance <= -(tickDelta * 3) {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("tps", p.Tps)
		d.Fail(p, data)
		d.balance = 0
		return true
	}

	avgTickTime := 0.0
	for _, tickTime := range d.tickTimes {
		avgTickTime += tickTime
	}
	avgTickTime /= float64(d.tickTimeCount)

	p.Dbg.Notify(
		player.DebugModeTimerA,
		true,
		"balance=%.4f delta=%.4fms avg=%.4f cTick=%d sTick=%d",
		d.balance,
		timeDiff,
		avgTickTime,
		p.ClientTick,
		p.ServerTick,
	)

	// This can occur if a user is attempting to use negative timer to increase their balance to a high amount,
	// to then use a high amount of timer after a period of time to bypass the check.
	if (math.Abs(50.0-avgTickTime) < 0.5 || p.ClientTick > p.ServerTick+4) && d.balance >= 150 && time.Since(p.LastServerTick).Milliseconds() < 100 {
		p.Dbg.Notify(
			player.DebugModeTimerA,
			true,
			"<red>timer balance reset due to conditions</red>",
		)
		d.balance = 0
	}

	return true
}
