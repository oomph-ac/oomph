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

	// This determines what tick we should rewind to get an entity position for lag compensation.
	// Lag compensation is limited to 250ms in this case, so we want two things:
	// 1) The tick we should rewind to should be no more than 5 ticks (250ms) in the past.
	// 2) The tick we should rewind to should not be higher than the current server tick
	tick, stick := p.clientTick.Load(), p.serverTick.Load()
	if tick < stick-5 {
		tick = stick - 5
	}
	if tick > stick {
		tick = stick
	}
	attackPos := p.mInfo.ServerPosition.Add(mgl64.Vec3{0, 1.62})

	if t, ok := p.SearchEntity(hit.TargetEntityRuntimeID); ok {
		rew := t.RewindPosition(tick)
		if rew == nil {
			return false
		}

		dist := game.AABBVectorDistance(t.AABB().Translate(rew.Position), attackPos)
		if dist > 3.1 {
			return false
		}

		// If a player's input mode is touch, then a raycast will not be performed to validate combat.
		// This is because touchscreen players have the ability to use touch controls (instead of split controls),
		// which would allow the player to attack another entity without actually looking at them.
		if p.inputMode != packet.InputModeTouch {
			targetAABB := t.AABB().Grow(0.1).Translate(rew.Position)
			dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())
			_, ok := trace.BBoxIntercept(targetAABB, attackPos, attackPos.Add(dV.Mul(3)))
			return ok
		}
	}

	return true
}
