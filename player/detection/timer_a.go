package detection

import (
	"math"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const timerA_tickSampleSize = 10

type TimerA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	tickTimes     []float64
	tickTimeCount uint8

	balance      float64
	lastTickTime time.Time
}

func New_TimerA(p *player.Player) *TimerA {
	return &TimerA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 15,
		},

		tickTimes: make([]float64, timerA_tickSampleSize),
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
		d.tickTimes[d.tickTimeCount] = timeDiff
		d.tickTimeCount++

		/* if !d.mPlayer.Alive {
			d.balance = 0
			return
		} */

		tickDelta := 1000 / float64(d.mPlayer.Tps)
		d.balance += timeDiff - tickDelta
		if d.balance <= -(tickDelta * 3) {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("tps", d.mPlayer.Tps)
			d.mPlayer.FailDetection(d, data)
			d.balance = 0
		}

		avgTickTime := 0.0
		for _, tickTime := range d.tickTimes {
			avgTickTime += tickTime
		}
		avgTickTime /= float64(d.tickTimeCount)

		d.mPlayer.Dbg.Notify(
			player.DebugModeTimerA,
			true,
			"balance=%.4f delta=%.4fms avg=%.4f cTick=%d sTick=%d",
			d.balance,
			timeDiff,
			avgTickTime,
			d.mPlayer.ClientTick,
			d.mPlayer.ServerTick,
		)

		// This can occur if a user is attempting to use negative timer to increase their balance to a high amount,
		// to then use a high amount of timer after a period of time to bypass the check.
		if (math.Abs(50.0-avgTickTime) < 0.5 || d.mPlayer.ClientTick > d.mPlayer.ServerTick+4) && d.balance >= 150 && time.Since(d.mPlayer.LastServerTick).Milliseconds() < 100 {
			d.mPlayer.Dbg.Notify(
				player.DebugModeTimerA,
				true,
				"<red>timer balance reset due to conditions</red>",
			)
			d.balance = 0
		}

		d.lastTickTime = curr
		if d.tickTimeCount == 10 {
			for i := 1; i < 10; i++ {
				d.tickTimes[i-1] = d.tickTimes[i]
			}
			d.tickTimeCount--
		}
	}
}
