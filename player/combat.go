package player

import (
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// validateCombat checks if the player's attack was valid for the tick. If combat is found to be legitimate, this function
// will return true. Note that if multiple attacks are recieved in a tick, this function will only validate the first
// processed in the tick, and any other hits will be ignored until next tick.
func (p *Player) validateCombat(hit *protocol.UseItemOnEntityTransactionData) bool {
	if p.gameMode != packet.GameTypeSurvival && p.gameMode != packet.GameTypeAdventure {
		return true
	}

	// Only validate one combat input per client tick - since we insinuate that combat should be
	// validated per tick (and not frame like the MC:BE client - the MC:JE client does combat on tick), there can only be one hit result.
	// This will also save server resources as it won't have to validate multiple hit results sent in one tick.
	if p.hasValidatedCombat {
		return false
	}
	p.hasValidatedCombat = true

	if t, ok := p.SearchEntity(hit.TargetEntityRuntimeID); ok {
		attackPos := p.mInfo.ServerPredictedPosition.Add(mgl64.Vec3{0, 1.62})
		dist := game.AABBVectorDistance(t.AABB().Translate(t.Position()), attackPos)
		if dist > 3.1 {
			return false
		}

		// If a player's input mode is touch, then a raycast will not be performed to validate combat.
		// This is because touchscreen players have the ability to use touch controls (instead of split controls),
		// which would allow the player to attack another entity without actually looking at them.
		if p.inputMode != packet.InputModeTouch {
			targetAABB := t.AABB().Grow(0.1).Translate(t.Position())
			dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())
			_, ok := trace.BBoxIntercept(targetAABB, attackPos, attackPos.Add(dV.Mul(3.01)))
			return ok
		}
	}

	return true
}
