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

	d.FailBuffer = 2
	d.MaxBuffer = 4
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

	_, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return true
	}

	combatHandler := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)
	if combatHandler.Phase != handler.CombatPhaseTransaction {
		return true
	}

	if combatHandler.ClosestRawDistance > 3 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("distance", game.Round32(combatHandler.ClosestRawDistance, 3))
		d.Fail(p, data)
	}

	d.Debuff(0.01)
	return true
}
