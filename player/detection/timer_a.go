package detection

import (
	"math"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const timerA_samples = 200

type TimerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	balance      float64
	lastTickTime time.Time
	tickTimes    []float64
	averages     []float64
}

func New_TimerA(p *player.Player) *TimerA {
	return &TimerA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 15,
		},

		tickTimes: make([]float64, 0, timerA_samples),
		averages:  make([]float64, 0, timerA_samples),

		lastTickTime: time.Now(),
	}
}

func (*TimerA) Type() string {
	return TYPE_TIMER
}

func (*TimerA) SubType() string {
	return "A"
}

func (*TimerA) Description() string {
	return "Detects if a player is simulating ahead of the server"
}

func (*TimerA) Punishable() bool {
	return true
}

func (d *TimerA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *TimerA) Detect(pk packet.Packet) {
	if _, ok := pk.(*packet.PlayerAuthInput); ok {
		curr := time.Now()
		timeDiff := float64(time.Since(d.lastTickTime).Microseconds()) / 1000
		d.tickTimes = append(d.tickTimes, timeDiff)

		avgTickTime := 0.0
		for _, tickTime := range d.tickTimes {
			avgTickTime += tickTime
		}
		avgTickTime /= float64(len(d.tickTimes))

		var avgTPS float64
		if avgTickTime > 0 {
			avgTPS = 20 * (50.0 / avgTickTime)
		}
		d.averages = append(d.averages, avgTPS)

		realTPS := 0.0
		for _, avg := range d.averages {
			realTPS += avg
		}
		realTPS /= float64(len(d.averages))

		tickDelta := 1000 / float64(d.mPlayer.Tps)
		d.balance += timeDiff - tickDelta
		if d.balance <= -(tickDelta * 3) {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("expected", d.mPlayer.Tps)
			data.Set("actual", avgTPS)
			d.mPlayer.FailDetection(d, data)
			d.balance = 0
		}

		d.mPlayer.Dbg.Notify(
			player.DebugModeTimerA,
			true,
			"balance=%.4f delta=%.4fms avg=%.4f real=%.2f cTick=%d sTick=%d",
			d.balance,
			timeDiff,
			avgTickTime,
			avgTPS,
			d.mPlayer.ClientTick,
			d.mPlayer.ServerTick,
		)

		// This can occur if a user is attempting to use negative timer to increase their balance to a high amount,
		// to then use a high amount of timer after a period of time to bypass the check.
		if (math.Abs(50.0-avgTickTime) < 0.5 || d.mPlayer.ClientTick > d.mPlayer.ServerTick+4) && d.balance >= 150 && time.Since(d.mPlayer.LastServerTick).Milliseconds() < 100 {
			d.mPlayer.Dbg.Notify(
				player.DebugModeTimerA,
				true,
				"timer balance reset due to conditions",
			)
			d.balance = 0
			d.resetTickTimes()
		}

		d.lastTickTime = curr
		d.shiftTickTimes()
	}
}

func (d *TimerA) shiftTickTimes() {
	if len(d.tickTimes) != len(d.averages) {
		panic("mismatched timer samples")
	}
	if len(d.tickTimes) != timerA_samples || len(d.averages) != timerA_samples {
		return
	}

	for i := 1; i < len(d.tickTimes); i++ {
		d.tickTimes[i-1] = d.tickTimes[i]
	}
	d.tickTimes = d.tickTimes[:timerA_samples-1]
	for i := 1; i < len(d.averages); i++ {
		d.averages[i-1] = d.averages[i]
	}
	d.averages = d.averages[:timerA_samples-1]
}

func (d *TimerA) resetTickTimes() {
	for i := 0; i < len(d.tickTimes); i++ {
		d.tickTimes[i] = 50.0
	}
	for i := 0; i < len(d.averages); i++ {
		d.averages[i] = 20.0
	}
}
