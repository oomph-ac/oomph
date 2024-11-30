package acknowledgement

import (
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/event"
)

// PlayerInitalized is an acknowledgment that is ran to signal when the player is ready for processing
type PlayerInitalized struct {
	mPlayer *player.Player
}

func NewPlayerInitalizedACK(p *player.Player) *PlayerInitalized {
	return &PlayerInitalized{mPlayer: p}
}

func (ack *PlayerInitalized) Run() {
	ack.mPlayer.Ready = true
}

// Latency is an acknowledgment that is ran to measure the full stack latency between
// Oomph and the member player.
type Latency struct {
	mPlayer *player.Player

	timeOf time.Time
	tickOf int64
}

func NewLatencyACK(p *player.Player, timeOf time.Time, tickOf int64) *Latency {
	return &Latency{
		mPlayer: p,
		timeOf:  timeOf,
		tickOf:  tickOf,
	}
}

func (ack *Latency) Run() {
	ack.mPlayer.StackLatency = time.Since(ack.timeOf)
	ack.mPlayer.ClientTick = ack.tickOf
	ack.mPlayer.Dbg.Notify(player.DebugModeLatency, true, "latency=%fms", game.Round64(float64(ack.mPlayer.StackLatency.Microseconds())/1000.0, 2))

	// Now that we have a response from the player, we can send a latency update to the remote server.
	ev := event.NewUpdateLatencyEvent(
		ack.mPlayer.Conn().Latency().Milliseconds()*2, // We multiply the conn's latency here by 2 to show an accurate RTT latency, which is what most players expect to be shown.
		ack.mPlayer.StackLatency.Milliseconds(),
	)
	ack.mPlayer.SendRemoteEvent(ev)
}

// UpdateSimRate is an acknowledgment that is ran to update the player's simulation rate.
type UpdateSimRate struct {
	mPlayer *player.Player
	mul     float32
}

func NewUpdateSimRateACK(p *player.Player, mul float32) *UpdateSimRate {
	return &UpdateSimRate{
		mPlayer: p,
		mul:     mul,
	}
}

func (ack *UpdateSimRate) Run() {
	if mgl32.FloatEqual(ack.mul, 1.0) {
		ack.mPlayer.Tps = 20.0
	} else {
		ack.mPlayer.Tps *= ack.mul
	}
}
