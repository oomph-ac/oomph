package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ReachA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	raycasts []float32
	run      bool
}

func New_ReachA(p *player.Player) *ReachA {
	d := &ReachA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1.01,
			MaxBuffer:     2.25,
			TrustDuration: 120 * player.TicksPerSecond,

			MaxViolations: 10,
		},

		raycasts: make([]float32, 0, 20),
	}
	p.ClientCombat().Hook(d.combatHook)

	return d
}

func (*ReachA) Type() string {
	return TYPE_REACH
}

func (*ReachA) SubType() string {
	return "A"
}

func (*ReachA) Description() string {
	return "Detects if a player's attack range exceeds the vanilla range."
}

func (*ReachA) Punishable() bool {
	return true
}

func (d *ReachA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *ReachA) Detect(pk packet.Packet) {
	if !d.run {
		return
	}
	d.run = false

	if len(d.raycasts) != 0 {
		var minDist, maxDist float32 = 1_000_000, -1
		for _, dist := range d.raycasts {
			if dist < minDist {
				minDist = dist
			}
			if dist > maxDist {
				maxDist = dist
			}
		}

		if minDist > 2.9 && maxDist > 3 {
			d.mPlayer.Log().Warnf("ReachA: min=%f max=%f", minDist, maxDist)
			d.mPlayer.FailDetection(d, nil)
		} else {
			d.mPlayer.PassDetection(d, 0.005)
		}
	}
}

func (d *ReachA) combatHook(cc player.CombatComponent) {
	d.raycasts = d.raycasts[:0]
	d.raycasts = append(d.raycasts, cc.Raycasts()...)
	d.run = true
}
