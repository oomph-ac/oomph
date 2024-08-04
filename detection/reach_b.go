package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDReachB = "oomph:reach_b"

type ReachB struct {
	BaseDetection
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
	// Full authoritative mode uses the rewind system, instead of completely lag compensating
	// for entity positions on the client.
	if p.CombatMode != player.AuthorityModeSemi {
		return true
	}

	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return true
	}

	cDat := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)
	if cDat.Phase != handler.CombatPhaseTicked {
		return true
	}

	if len(cDat.NonRaycastResults) == 0 {
		return true
	}

	var (
		minDist float32 = 14
		maxDist float32 = -1
	)
	for _, dist := range cDat.NonRaycastResults {
		if dist < minDist {
			minDist = dist
		}
		if dist > maxDist {
			maxDist = dist
		}
	}

	p.Dbg.Notify(
		player.DebugModeCombat,
		true,
		"Reach (B): min=%f max=%f",
		game.Round32(minDist, 4), game.Round32(maxDist, 4),
	)

	// TODO: Adjust like in Reach (A)?
	if minDist > 3 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("dist", game.Round32(minDist, 3))
		d.Fail(p, data)
		return true
	}

	d.Debuff(0.01)
	return true
}
