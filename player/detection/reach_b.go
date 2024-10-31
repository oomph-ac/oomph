package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ReachB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	distances []float32
	run       bool
}

func New_ReachB(p *player.Player) *ReachB {
	d := &ReachB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1.01,
			MaxBuffer:     3,
			TrustDuration: 30 * player.TicksPerSecond,

			MaxViolations: 5,
		},

		distances: make([]float32, 0, 10),
	}
	p.ClientCombat().Hook(func(cc player.CombatComponent) {
		d.distances = d.distances[:0]
		for _, dist := range cc.Raws() {
			d.distances = append(d.distances, dist)
		}

		d.run = true
	})

	return d
}

func (*ReachB) Type() string {
	return TYPE_REACH
}

func (*ReachB) SubType() string {
	return "A"
}

func (*ReachB) Description() string {
	return "Detects if a player's attack range exceeds the vanilla range."
}

func (*ReachB) Punishable() bool {
	return true
}

func (d *ReachB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *ReachB) Detect(pk packet.Packet) {
	if !d.run {
		return
	}
	d.run = false

	if len(d.distances) != 0 {
		var minDist float32 = 1_000_000
		for _, dist := range d.distances {
			if dist < minDist {
				minDist = dist
			}
		}

		if minDist >= 3.01 {
			d.mPlayer.Log().Warnf("ReachB: min=%f", minDist)
			d.mPlayer.FailDetection(d, nil)
		} else {
			d.mPlayer.PassDetection(d, 0.005)
		}
	}
}
