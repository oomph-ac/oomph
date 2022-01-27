package check

import (
	"github.com/df-mc/dragonfly/server/entity/physics/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"math"
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
				if t, ok := processor.Entity(data.TargetEntityRuntimeID); ok {
					var add float32 = 1.54
					if !s.HasFlag(session.FlagSneaking) {
						add = 1.62
					}
					r.attackedEntity = data.TargetEntityRuntimeID
					r.attackPos = data.Position.Sub(mgl32.Vec3{0, 1.62}).Add(mgl32.Vec3{0, add})
					if r.inputMode == packet.InputModeTouch {
						dist := omath.AABBVectorDistance(t.AABB, omath.Vec32To64(r.attackPos))
						processor.Debug(r, map[string]interface{}{"dist": dist})
						if dist > 3.1 {
							if r.Buff(r.updateAndGetViolationAfterTicks(processor.Tick(), 300)) >= 5 {
								processor.Flag(r, map[string]interface{}{"dist": omath.Round(dist, 4)})
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
			if t, ok := processor.Entity(r.attackedEntity); ok {
				rot := processor.Location().Rotation
				dv := omath.DirectionVectorFromValues(rot.Y(), rot.X())
				aabb := t.AABB.Extend(mgl64.Vec3{0.1, 0.1, 0.1})
				if !aabb.IntersectsWith(processor.Session().GetEntityData().AABB) {
					vec64AttackPos := omath.Vec32To64(r.attackPos)
					if raycast, ok := trace.AABBIntercept(aabb, vec64AttackPos, vec64AttackPos.Add(dv.Mul(20))); ok {
						dist := omath.AABBVectorDistance(raycast.AABB(), vec64AttackPos)
						processor.Debug(r, map[string]interface{}{"raycast": dist})
						if dist > 3.04 {
							if r.Buff(r.updateAndGetViolationAfterTicks(processor.Tick(), 100), 3.1) >= 3 {
								processor.Flag(r, map[string]interface{}{"raycast": omath.Round(dist, 2)})
							}
						} else {
							r.Buff(-0.01)
							r.violations = math.Max(r.violations-0.0075, 0)
						}
					}
				}
			}
			r.awaitingTick = true
		}
	}
}
