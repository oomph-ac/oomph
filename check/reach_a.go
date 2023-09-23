package check

import (
	"math"

	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const interpolatedFrames float32 = 15

type ReachA struct {
	eid uint64
	run bool
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
	return 10
}

func (r *ReachA) Process(p Processor, pk packet.Packet) bool {
	if p.CombatMode() != utils.ModeSemiAuthoritative {
		return false
	}

	if tpk, ok := pk.(*packet.InventoryTransaction); ok {
		d, ok := tpk.TransactionData.(*protocol.UseItemOnEntityTransactionData)
		if !ok {
			return false
		}

		if d.ActionType != protocol.UseItemOnEntityActionAttack {
			return false
		}

		r.eid = d.TargetEntityRuntimeID
		r.run = true

		return false
	}

	if i, ok := pk.(*packet.PlayerAuthInput); ok && r.run {
		defer func() {
			r.run = false
		}()

		if i.InputMode != packet.InputModeMouse {
			return false
		}

		e, ok := p.SearchEntity(r.eid)
		if !ok { // The entity does not exist and we cannot do the reach check.
			return false
		}

		bb := e.AABB().Translate(e.LastPosition()).Grow(0.15)
		pe := p.Entity()

		atkPos := pe.LastPosition().Add(mgl32.Vec3{0, 1.62})
		cDv, lDv := game.DirectionVector(pe.Rotation()[2], pe.Rotation()[0]),
			game.DirectionVector(pe.LastRotation()[2], pe.LastRotation()[0])
		dvDelta := cDv.Sub(lDv).Mul(1 / interpolatedFrames)
		uDv := lDv

		minDist, valid := float32(6900.0), false

		for y := float32(0.0); y < interpolatedFrames; y++ {
			if y != 0 {
				uDv = uDv.Add(dvDelta)
			}

			result, ok := trace.BBoxIntercept(bb, atkPos, atkPos.Add(uDv.Mul(14.0)))
			if !ok {
				continue
			}

			dist := result.Position().Sub(atkPos).Len()
			if dist > 14 { // This is impossible as we only traversed 14 blocks.
				continue
			}

			valid = true
			if dist < minDist {
				minDist = dist
			}
		}

		if !valid {
			return false
		}

		if minDist < 3.01 {
			r.Buff(-0.0125)
			r.violations -= math.Max(r.violations-0.002, 0)
			return false
		}

		if r.Buff(1, 3.3) < 3 {
			return false
		}

		p.Flag(r, 1, map[string]any{
			"dist": game.Round32(minDist, 2),
		})
	}

	return false
}
