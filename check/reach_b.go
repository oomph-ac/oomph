package check

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ReachB struct {
	basic
}

func NewReachB() *ReachB {
	return &ReachB{}
}

func (*ReachB) Name() (string, string) {
	return "Reach", "B"
}

func (*ReachB) Description() string {
	return "This checks if a player's combat is invalid by checking raw distance"
}

func (*ReachB) MaxViolations() float64 {
	return math.MaxFloat64
}

func (r *ReachB) Process(p Processor, pk packet.Packet) bool {
	t, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return false
	}

	if p.CombatMode() != utils.ModeSemiAuthoritative {
		return false
	}

	d, ok := t.TransactionData.(*protocol.UseItemOnEntityTransactionData)
	if !ok {
		return false
	}

	if d.ActionType != protocol.UseItemOnEntityActionAttack {
		return false
	}

	e, ok := p.SearchEntity(d.TargetEntityRuntimeID)
	if !ok {
		return false
	}

	bb := e.AABB().Translate(e.Position()).Grow(0.1)
	dist := game.AABBVectorDistance(bb, p.Entity().Position().Add(mgl32.Vec3{0, 1.62}))

	if dist < 3.1 {
		r.violations = math.Max(r.violations-0.02, 0)
		return false
	}

	if r.Buff(1, 10) < 5 {
		return false
	}

	p.Flag(r, r.violationAfterTicks(p.ClientFrame(), 300), map[string]any{
		"dist": game.Round32(dist, 2),
	})

	return true
}
