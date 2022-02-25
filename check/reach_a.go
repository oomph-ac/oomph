package check

import (
	"github.com/justtaldevelops/oomph/game"
	"math"

	"github.com/df-mc/dragonfly/server/entity/physics/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ReachA checks if a player has an abnormal amount of reach.
type ReachA struct {
	basic
	awaitingTick   bool
	inputMode      uint32
	attackPos      mgl32.Vec3
	attackedEntity uint64
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
func (r *ReachA) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			if processor.GameMode() == packet.GameTypeSurvival || processor.GameMode() == packet.GameTypeAdventure {
				offset := float32(1.62)
				if processor.Sneaking() {
					offset = 1.54
				}

				r.attackedEntity = data.TargetEntityRuntimeID
				r.attackPos = data.Position.Sub(mgl32.Vec3{0, 1.62}).Add(mgl32.Vec3{0, offset})
				if t, ok := processor.SearchEntity(data.TargetEntityRuntimeID); ok { // todo: && $target->teleportTicks >= 40
					dist := game.AABBVectorDistance(t.AABB().GrowVec3(mgl64.Vec3{0.1, 0.1, 0.1}), game.Vec32To64(r.attackPos))
					if dist > 3.15 {
						if r.Buff(1, 10) >= 5 {
							processor.Flag(r, r.updateAndGetViolationAfterTicks(processor.ClientTick(), 600), map[string]interface{}{
								"Distance": game.Round(dist, 4),
								"Type":     "Raw",
							})
						}
					} else {
						r.Buff(-0.05)
						r.violations = math.Max(r.violations-0.01, 0)
					}
					if r.inputMode != packet.InputModeTouch {
						r.awaitingTick = true
					}
				}
			}
		}
	case *packet.PlayerAuthInput:
		r.inputMode = pk.InputMode
		if r.awaitingTick {
			if t, ok := processor.SearchEntity(r.attackedEntity); ok && t.Player() {
				e := processor.Entity()
				rot := e.Rotation()
				dv := game.DirectionVector(rot.Y(), rot.X())

				aabb := e.AABB().Translate(e.LastPosition())
				targetAABB := t.AABB().Translate(t.LastPosition()).Grow(0.1)

				if !aabb.IntersectsWith(targetAABB) {
					vec64AttackPos := game.Vec32To64(r.attackPos)
					if ray, ok := trace.AABBIntercept(aabb, vec64AttackPos, vec64AttackPos.Add(dv.Mul(14.0))); ok {
						dist := ray.Position().Sub(vec64AttackPos).Len()
						if dist >= 3.1 && math.Abs(dist-game.AABBVectorDistance(targetAABB, vec64AttackPos)) < 0.4 {
							if r.Buff(1, 10) >= 3 {
								processor.Flag(r, r.updateAndGetViolationAfterTicks(processor.ClientTick(), 600), map[string]interface{}{
									"Distance": game.Round(dist, 2),
									"Type":     "Raycast",
								})
							}
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
}
