package check

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube/trace"
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

		attackPos := game.Vec32To64(r.attackData.Position)

		e, ok := p.SearchEntity(r.attackData.TargetEntityRuntimeID)
		if !ok {
			return false
		}

		dist1, entPos, dPos := 6969.0, e.LastPosition(), e.Position().Sub(e.LastPosition()).Mul(1.0/10.0)
		for i := 0.0; i < 10; i++ {
			if i != 0 {
				entPos = entPos.Add(dPos)
			}

			bb := e.AABB().Translate(entPos).Grow(0.1)
			dist := game.AABBVectorDistance(bb, attackPos)
			if dist1 > dist {
				dist1 = dist
			}
		}

		if dist1 > 3.1 && r.Buff(r.violationAfterTicks(p.ClientFrame(), 600), 6) >= 5 {
			p.Flag(r, 1, map[string]any{
				"rawDist": game.Round(dist1, 4),
			})
			r.cancelNext = true
		} else if dist1 <= 3.1 {
			r.Buff(-0.01, 10)
		}

		if i.InputMode == packet.InputModeTouch {
			return false
		}

		dist2, valid := 6969.0, false
		rot := game.DirectionVector(p.Entity().LastRotation().Z(), p.Entity().LastRotation().X())
		dRot := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X()).Sub(rot).Mul(1.0 / 10.0)
		entPos = e.LastPosition()

		for i := 0.0; i < 10; i++ {
			if i != 0 {
				entPos = entPos.Add(dPos)
			}

			for x := 0; x < 10; x++ {
				if x != 0 {
					rot = rot.Add(dRot)
				}
				bb := e.AABB().Translate(entPos).Grow(0.1)
				result, ok := trace.BBoxIntercept(bb, attackPos, attackPos.Add(rot.Mul(7.0)))
				if !ok {
					continue
				}

				valid = true
				dist := result.Position().Sub(attackPos).LenSqr()
				if dist2 > dist {
					dist2 = dist
				}
			}
		}

		if !valid {
			return false
		}

		// 3^2 = 9
		if dist2 <= 9 {
			r.Buff(-0.01, 6)
			return false
		}

		if r.Buff(r.violationAfterTicks(p.ClientFrame(), 600), 6) >= 3 {
			p.Flag(r, 1, map[string]any{
				"dist": game.Round(math.Sqrt(dist2), 4),
			})
			r.cancelNext = true
		}
	}

	return false
}
