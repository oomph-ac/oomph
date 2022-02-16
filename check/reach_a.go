package check

import (
	"math"

	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/df-mc/dragonfly/server/entity/physics/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ReachA checks if a player has an abnormal amount of reach.
type ReachA struct {
	check
	awaitingTick   bool
	inputMode      uint32
	attackPos      mgl32.Vec3
	attackedEntity uint64
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
func (*ReachA) MaxViolations() uint32 {
	return 15
}

// Punishment ...
func (*ReachA) Punishment() punishment.Punishment {
	return punishment.Ban()
}

// Process ...
func (r *ReachA) Process(processor Processor, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if data, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && data.ActionType == protocol.UseItemOnEntityActionAttack {
			s := processor.Session()
			if s.Gamemode != 1 {
				var add float32 = 1.54
				if !s.HasFlag(session.FlagSneaking) {
					add = 1.62
				}
				r.attackedEntity = data.TargetEntityRuntimeID
				r.attackPos = data.Position.Sub(mgl32.Vec3{0, 1.62}).Add(mgl32.Vec3{0, add})
				if t, ok := processor.Entity(data.TargetEntityRuntimeID); ok { // todo: && $target->teleportTicks >= 40
					dist := omath.AABBVectorDistance(t.AABB.GrowVec3(mgl64.Vec3{0.1, 0.1, 0.1}), omath.Vec32To64(r.attackPos))
					//processor.Debug(r, map[string]interface{}{"dist": dist, "type": "raw"})
					if dist > 3.15 {
						if r.Buff(r.updateAndGetViolationAfterTicks(processor.ClientTick(), 300)) >= 5 {
							processor.Flag(r, map[string]interface{}{"dist": omath.Round(dist, 4), "type": "raw"})
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
			if t, ok := processor.Entity(r.attackedEntity); ok && t.IsPlayer {
				rot := processor.Location().Rotation
				dv := omath.DirectionVectorFromValues(rot.Y(), rot.X())
				width, height := (t.AABB.Width()/2)+0.1, t.AABB.Height()+0.1
				aabb := physics.NewAABB(
					t.LastPosition.Sub(mgl64.Vec3{width, 0.1, width}),
					t.LastPosition.Add(mgl64.Vec3{width, height, width}),
				)
				if !aabb.IntersectsWith(processor.Session().GetEntityData().AABB) {
					vec64AttackPos := omath.Vec32To64(r.attackPos)
					if raycast, ok := trace.AABBIntercept(aabb, vec64AttackPos, vec64AttackPos.Add(dv.Mul(14.0))); ok {
						dist := raycast.Position().Sub(vec64AttackPos).Len()
						//processor.Debug(r, map[string]interface{}{"dist": dist, "type": "raycast"})
						if dist >= 3.1 && math.Abs(dist-omath.AABBVectorDistance(aabb, vec64AttackPos)) < 0.4 {
							if r.Buff(r.updateAndGetViolationAfterTicks(processor.ClientTick(), 100), 3.1) >= 3 {
								processor.Flag(r, map[string]interface{}{"dist": omath.Round(dist, 2), "type": "raycast"})
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
