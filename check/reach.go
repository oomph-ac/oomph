package check

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ReachA struct {
	attackData *protocol.UseItemOnEntityTransactionData
	cancelNext bool
	basic
}

func NewReachA() *ReachA {
	return &ReachA{}
}

func (*ReachA) Name() (string, string) {
	return "Reach", "A"
}

func (*ReachA) Description() string {
	return "This checks if a player's combat range is invalid."
}

func (*ReachA) MaxViolations() float64 {
	return 25
}

func (r *ReachA) Process(p Processor, pk packet.Packet) bool {
	if p.CombatMode() != utils.ModeSemiAuthoritative {
		return false
	}

	if t, ok := pk.(*packet.InventoryTransaction); ok {
		d, ok := t.TransactionData.(*protocol.UseItemOnEntityTransactionData)
		if !ok {
			return false
		}

		if d.ActionType != protocol.UseItemOnEntityActionAttack {
			return false
		}

		if r.cancelNext {
			r.cancelNext = false
			return true
		}

		r.attackData = d
	} else if i, ok := pk.(*packet.PlayerAuthInput); ok && r.attackData != nil {
		defer func() {
			r.attackData = nil
		}()

		attackPos := p.Entity().Position().Add(mgl64.Vec3{0, 1.62})

		e, ok := p.SearchEntity(r.attackData.TargetEntityRuntimeID)
		if !ok {
			return false
		}

		bb := e.AABB().Translate(e.CurrentPosition()).Grow(0.1)
		dist := game.AABBVectorDistance(bb, attackPos)
		if dist > 3.1 {
			if r.Buff(1, 6) >= 5 {
				p.Flag(r, r.violationAfterTicks(p.ClientFrame(), 600), map[string]any{
					"rawDist": game.Round(dist, 4),
				})
				r.cancelNext = true
			}
		} else {
			r.Buff(-0.01, 10)
		}

		if i.InputMode == packet.InputModeTouch {
			return false
		}

		dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())
		result, ok := trace.BBoxIntercept(bb, attackPos, attackPos.Add(dV.Mul(7)))

		if !ok {
			return false
		}

		dist2 := result.Position().Sub(attackPos).Len()
		if dist2 > 3.01 && math.Abs(dist-dist2) < 0.4 {
			if r.Buff(1, 6) >= 3 {
				p.Flag(r, r.violationAfterTicks(p.ClientFrame(), 600), map[string]any{
					"dist": game.Round(dist2, 4),
				})
				r.cancelNext = true
			}
		} else {
			r.Buff(-0.01, 6)
		}
	}

	return false
}
