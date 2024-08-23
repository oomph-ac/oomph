package detection

import (
	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/game"
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

	var (
		minDist float32 = 14
		maxDist float32 = -1
	)

	avg := game.Mean32(combatHandler.RaycastResults)
	for _, result := range combatHandler.RaycastResults {
		minDist = math32.Min(minDist, result)
		maxDist = math32.Max(maxDist, result)
	}

	p.Dbg.Notify(
		player.DebugModeCombat,
		true,
		"Reach (A): minDist=%f maxDist=%f avg=%f",
		game.Round32(minDist, 4),
		game.Round32(maxDist, 4),
		game.Round32(avg, 4),
	)

	if minDist > 2.9 && maxDist > 3 {
		p.Log().Warnf("ReachA: min=%f max=%f", minDist, maxDist)
		d.Fail(p, nil)
		return true
	}

	d.Debuff(0.005)
	return true
}
