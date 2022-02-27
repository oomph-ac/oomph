package check

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/game"
	"math"

	"github.com/df-mc/dragonfly/server/entity/physics/trace"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ReachA checks if a player has an abnormal amount of reach.
type ReachA struct {
	basic
	awaitingTick   bool
	inputMode      uint32
	attackPos      mgl64.Vec3
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
				r.attackedEntity = data.TargetEntityRuntimeID
				r.attackPos = game.Vec32To64(data.Position)
				if processor.Sneaking() {
					r.attackPos[1] -= 0.08
				}
				if t, ok := processor.SearchEntity(data.TargetEntityRuntimeID); ok && t.TeleportationTicks() >= 40 {
					if r.inputMode == packet.InputModeTouch {
						dist := game.AABBVectorDistance(t.AABB().Translate(t.Position()), r.attackPos)
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
					} else {
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
				dv := game.DirectionVector(rot.Z(), rot.X())

				aabb := e.AABB().Translate(e.LastPosition())
				targetAABB := t.AABB().Grow(0.1).Translate(t.LastPosition())

				if !aabb.IntersectsWith(targetAABB) {
					if ray, ok := trace.AABBIntercept(targetAABB, r.attackPos, r.attackPos.Add(dv.Mul(14.0))); ok {
						dist := world.Distance(ray.Position(), r.attackPos)
						if dist >= 3.1 && math.Abs(dist-game.AABBVectorDistance(targetAABB, r.attackPos)) < 0.4 {
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
