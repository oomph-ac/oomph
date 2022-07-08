package player

import (
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (p *Player) validateCombat(pk *packet.InventoryTransaction) bool {
	hit, _ := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData)
	if t, ok := p.SearchEntity(hit.TargetEntityRuntimeID); ok && !p.Teleporting() {
		attackPos := p.mInfo.ServerPredictedPosition.Add(mgl64.Vec3{0, 1.62})
		dist := game.AABBVectorDistance(t.AABB().Translate(t.Position()), attackPos)
		if dist > 3.15 {
			return false
		}

		if p.inputMode != packet.InputModeTouch {
			targetAABB := t.AABB().Grow(0.1).Translate(t.Position())
			dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())
			dist, valid := 0.0, false
			if ray, ok := trace.BBoxIntercept(targetAABB, attackPos, attackPos.Add(dV.Mul(14))); ok {
				dist = ray.Position().Sub(attackPos).Len()
				valid = true
			}
			if !valid || dist > 3.01 {
				return false
			}
		}
	}

	return true
}
