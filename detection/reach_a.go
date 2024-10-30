package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDReachA = "oomph:reach_a"

type ReachA struct {
	BaseDetection
	initalized bool
}

func NewReachA() *ReachA {
	d := &ReachA{}
	d.Type = "Reach"
	d.SubType = "A"

	d.Description = "Detects if a player's attack range exceeds 3 blocks."
	d.Punishable = true

	d.MaxViolations = 10
	d.trustDuration = 60 * player.TicksPerSecond

	d.FailBuffer = 1.01
	d.MaxBuffer = 1.5

	return d
}

func (d *ReachA) ID() string {
	return DetectionIDReachA
}

func (d *ReachA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if !d.initalized {
		d.initalized = true
		p.ClientCombat().Hook(func(cc player.CombatComponent) {
			if len(cc.Raycasts()) == 0 || (p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure) {
				return
			}

			var minDist, maxDist float32 = 1_000_000, -1
			for _, rayDist := range cc.Raycasts() {
				if rayDist < minDist {
					minDist = rayDist
				}
				if rayDist > maxDist {
					maxDist = rayDist
				}
			}

			if minDist > 2.9 && maxDist > 3 {
				p.Log().Warnf("ReachA: min=%f max=%f", minDist, maxDist)
				d.Fail(p, nil)
				return
			}

			d.Debuff(0.005)
		})
	}

	return true
}
