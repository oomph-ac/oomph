package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDReachB = "oomph:reach_b"

type ReachB struct {
	BaseDetection
}

func NewReachB() *ReachB {
	d := &ReachB{}
	d.Type = "Reach"
	d.SubType = "B"

	d.Description = "Checks if shortest distance from player's eye height to entity bounding box exceeds 3 blocks."
	d.Punishable = true

	d.MaxViolations = 5
	d.trustDuration = 30 * player.TicksPerSecond

	d.FailBuffer = 2
	d.MaxBuffer = 4
	return d
}

func (d *ReachB) ID() string {
	return DetectionIDReachB
}

func (d *ReachB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	// Full authoritative mode uses the rewind system, instead of completely lag compensating
	// for entity positions on the client.
	if p.CombatMode != player.AuthorityModeSemi {
		return true
	}

	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return true
	}

	trns, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return true
	}

	dat, ok := trns.TransactionData.(*protocol.UseItemOnEntityTransactionData)
	if !ok {
		return true
	}

	if dat.ActionType != protocol.UseItemOnEntityActionAttack {
		return true
	}

	entity := p.Handler(handler.HandlerIDEntities).(*handler.EntityHandler).Find(dat.TargetEntityRuntimeID)
	if entity == nil {
		return true
	}

	// TODO: Proper combat handler so that reach detections only need to access data already calculated.
	movementHandler := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if movementHandler.TicksSinceTeleport <= 10 {
		return true
	}

	offset := float32(1.62)
	if movementHandler.Sneaking {
		offset = 1.54
	}

	// Get the two possible attack positions.
	startAttackPos := movementHandler.PrevClientPosition.Add(mgl32.Vec3{0, offset})
	endAttackPos := movementHandler.ClientPosition.Add(mgl32.Vec3{0, offset})

	bb := entity.Box(entity.Position).Grow(0.1)

	// Get the closest point to the bounding box from both attack positions, and then find
	// the mininum distance between the two.
	point1 := game.ClosestPointToBBox(startAttackPos, bb)
	point2 := game.ClosestPointToBBox(endAttackPos, bb)
	dist := math32.Min(startAttackPos.Sub(point1).Len(), endAttackPos.Sub(point2).Len())

	// If the distance is greater than 3 blocks, the player is using blatant reach.
	if dist > 3 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("distance", dist)
		d.Fail(p, data)
	}

	d.Debuff(0.02)
	return true
}
