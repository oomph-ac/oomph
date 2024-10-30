package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDReachB = "oomph:reach_b"

type ReachB struct {
	BaseDetection
	initalized bool
}

func NewReachB() *ReachB {
	d := &ReachB{}
	d.Type = "Reach"
	d.SubType = "B"

	d.Description = "Checks if shortest distance from player's eye height to entity bounding box exceeds 3 blocks."
	d.Punishable = true

	d.MaxViolations = 5
	d.trustDuration = 30 * player.TicksPerSecond

	d.FailBuffer = 3
	d.MaxBuffer = 6
	return d
}

func (d *ReachB) ID() string {
	return DetectionIDReachB
}

func (d *ReachB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if !d.initalized {
		d.initalized = true
		p.ClientCombat().Hook(func(cc player.CombatComponent) {
			if len(cc.Raws()) == 0 || (p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure) {
				return
			}

			var minDist float32 = 1_000_000
			for _, rayDist := range cc.Raws() {
				if rayDist < minDist {
					minDist = rayDist
				}
			}

			if minDist > 3.01 {
				d.Fail(p, nil)
			} else {
				d.Debuff(0.005)
			}
		})
	}

	return true
}
