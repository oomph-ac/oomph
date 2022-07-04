package check

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ReachA checks if a player has an abnormal amount of reach.
type ReachA struct {
	awaitingTick   bool
	inputMode      uint32
	attackedEntity uint64
	attackPos      mgl64.Vec3
	basic
}

// NewReachA creates a new ReachA check.
func NewReachA() *ReachA {
	return &ReachA{}
}

// Name ...
func (*ReachA) Name() (string, string) {
	return "Reach", "A"
}

// Description ...
func (*ReachA) Description() string {
	return "This checks if a player has an abnormal amount of reach."
}

// MaxViolations ...
func (*ReachA) MaxViolations() float64 {
	return 15
}

// Process ...
func (r *ReachA) Process(p Processor, pk packet.Packet) bool {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			if p.GameMode() == packet.GameTypeSurvival || p.GameMode() == packet.GameTypeAdventure {
				r.attackedEntity = data.TargetEntityRuntimeID
				r.attackPos = game.Vec32To64(data.Position)
				// TODO: When added to Bedrock, account for sneaking AABB.
				if t, ok := p.SearchEntity(data.TargetEntityRuntimeID); ok && !p.Teleporting() {
					if r.inputMode != packet.InputModeTouch {
						r.awaitingTick = true
					}
					dist := game.AABBVectorDistance(t.AABB().Translate(t.Position()), r.attackPos)
					if dist > 3.15 {
						/*if r.Buff(1, 10) >= 5 {
							p.Flag(r, r.violationAfterTicks(p.ClientTick(), 600), map[string]interface{}{
								"Distance": game.Round(dist, 4),
								"Type":     "Raw",
							})
							return true
						}*/
						return true
					} else {
						r.Buff(-0.05)
						r.violations = math.Max(r.violations-0.01, 0)
					}
				}
			}
		}
	case *packet.PlayerAuthInput:
		r.inputMode = pk.InputMode
		if r.awaitingTick {
			if t, ok := p.SearchEntity(r.attackedEntity); ok && t.Player() {
				e := p.Entity()
				cRot, lRot := e.Rotation(), e.LastRotation()
				cDv, lDv := game.DirectionVector(cRot.Z(), cRot.X()), game.DirectionVector(lRot.Z(), lRot.X())
				cPos, lPos := p.Entity().Position().Add(mgl64.Vec3{0, 1.62, 0}), r.attackPos
				cEntPos, lEntPos := t.Position(), t.LastPosition()
				/* if p.Sneaking() {
					cPos[1] -= 0.08
				} */

				targetAABB := t.AABB().Grow(0.1).Translate(t.LastPosition())

				if !e.AABB().Translate(e.LastPosition()).IntersectsWith(targetAABB) {
					dvDiff := cDv.Sub(lDv)
					posDiff := cPos.Sub(lPos)
					entPosDiff := cEntPos.Sub(lEntPos)
					minDist, valid := 69000.0, false
					//maxDist := 3.1
					for i := 0; i <= 30; i++ {
						uDv := lDv
						uPos := lPos
						uEntPos := lEntPos
						if i != 0 {
							uDv = uDv.Add(dvDiff.Mul(float64(1 / i)))
							uPos = uPos.Add(posDiff.Mul(float64(1 / i)))
							uEntPos = uEntPos.Add(entPosDiff.Mul(float64(1 / i)))
						}
						uAABB := t.AABB().Translate(uEntPos)
						if ray, ok := trace.BBoxIntercept(uAABB, uPos, uPos.Add(uDv.Mul(14))); ok {
							minDist = math.Min(minDist, ray.Position().Sub(uPos).Len())
							valid = true
						}
					}
					if valid {
						if minDist >= 3 && math.Abs(minDist-game.AABBVectorDistance(targetAABB, r.attackPos)) < 0.4 {
							/* if minDist >= maxDist && r.Buff(1, 6) >= 3 {
								p.Flag(r, r.violationAfterTicks(p.ClientTick(), 600), map[string]any{
									"Distance": game.Round(minDist, 2),
									"Type":     "Raycast",
								})
							} */
							return true
						} else {
							r.Buff(-0.01)
							r.violations = math.Max(r.violations-0.0075, 0)
						}
					}
				}
			}
			r.awaitingTick = false
		}
	}

	return false
}
