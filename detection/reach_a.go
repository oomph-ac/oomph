package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDReachA = "oomph:reach_a"

type ReachA struct {
	BaseDetection
}

func NewReachA() *ReachA {
	d := &ReachA{}
	d.Type = "Reach"
	d.SubType = "A"

	d.Description = "Detects if a player's attack range exceeds 3 blocks."
	d.Punishable = true

	d.MaxViolations = 15
	d.trustDuration = 90 * player.TicksPerSecond

	d.FailBuffer = 1.001
	d.MaxBuffer = 2.5
	return d
}

func (d *ReachA) ID() string {
	return DetectionIDReachA
}

func (d *ReachA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	// Full authoritative mode uses the rewind system, instead of completely lag compensating
	// for entity positions on the client.
	if p.CombatMode != player.AuthorityModeSemi {
		return true
	}

	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return true
	}

	combatHandler := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)
	if combatHandler.Phase != handler.CombatPhaseTicked {
		return true
	}

	if len(combatHandler.RaycastResults) == 0 {
		return true
	}

	// This gets the mininum distance calculated by game.ClosestPointToBBoxDirectional()
	minDirectionalDist := float32(1000)
	for _, result := range combatHandler.ClosestDirectionalResults {
		minDirectionalDist = math32.Min(minDirectionalDist, result)
	}

	// This calculates the average distance of all the raycast results.
	total, count := float32(0), float32(0)
	for _, result := range combatHandler.RaycastResults {
		total += result
		count++
	}

	avgDist := total / count
	if minDirectionalDist >= 3 && avgDist >= 3 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("distance", minDirectionalDist)
		d.Fail(p, data)

		return true
	}

	d.Debuff(0.004)
	return true
}
