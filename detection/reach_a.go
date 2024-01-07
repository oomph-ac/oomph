package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDReachA = "oomph:reach_a"
const interpolationIncrement float32 = 1 / 10.0

type ReachA struct {
	BaseDetection

	run            bool
	targetedEntity uint64

	startAttackPos mgl32.Vec3
}

func NewReachA() *ReachA {
	d := &ReachA{}
	d.Type = "Reach"
	d.SubType = "A"

	d.Description = "Detects if a player's attack range exceeds 3 blocks."
	d.Punishable = true

	d.MaxViolations = 15
	d.trustDuration = 90 * player.TicksPerSecond

	d.FailBuffer = 1.001
	d.MaxBuffer = 2.5
	return d
}

func (d *ReachA) ID() string {
	return DetectionIDReachA
}

func (d *ReachA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	// Full authoritative mode uses the rewind system, instead of completely lag compensating
	// for entity positions on the client
	if p.CombatMode != player.AuthorityModeSemi {
		return true
	}

	if p.GameMode != packet.GameTypeSurvival && p.GameMode != packet.GameTypeAdventure {
		return true
	}

	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		// We already have an attack queued, so we can ignore this.
		if d.run {
			return true
		}

		dat, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData)
		if !ok {
			return true
		}

		if dat.ActionType != protocol.UseItemOnEntityActionAttack {
			return true
		}

		entity := p.Handler(handler.HandlerIDEntities).(*handler.EntityHandler).FindEntity(dat.TargetEntityRuntimeID)
		if entity == nil {
			return true
		}

		movementHandler := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
		d.startAttackPos = movementHandler.PrevClientPosition
		d.targetedEntity = dat.TargetEntityRuntimeID
		d.run = true
	case *packet.PlayerAuthInput:
		defer func() {
			d.run = false
		}()

		if !d.run {
			return true
		}

		if pk.InputMode != packet.InputModeMouse {
			return true
		}

		entity := p.Handler(handler.HandlerIDEntities).(*handler.EntityHandler).FindEntity(d.targetedEntity)
		if entity == nil {
			return true
		}

		movementHandler := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
		if movementHandler.TicksSinceTeleport <= 10 {
			return true
		}

		offset := float32(1.62)
		if movementHandler.Sneaking {
			offset = 1.54
		}

		// Get the current and previous rotation of the player. This will be used for interpolation
		// possible rotations in between.
		startRotation := movementHandler.PrevRotation
		endRotation := movementHandler.Rotation
		attackRotationDelta := endRotation.Sub(startRotation)

		// Do not attempt a raycast if there is a significant change in the player's yaw. This is because
		// we only want to perform a fixed number of raycasts, and if the player has a high change in rotation,
		// the raycasts could lead to unreliable results.
		if attackRotationDelta.Len() >= 20 {
			return true
		}

		// These will be used primarily for calculating the closest point from the attack position
		// to the entity's hitbox.
		startAttackDirection := game.DirectionVector(startRotation.Z(), startRotation.X())
		endAttackDirection := game.DirectionVector(endRotation.Z(), endRotation.X())

		// Get the current and previous position of the player. This will be used for interpolation.
		startAttackPos := d.startAttackPos.Add(mgl32.Vec3{0, offset})
		endAttackPos := movementHandler.PrevClientPosition.Add(mgl32.Vec3{0, offset})
		attackPosDelta := endAttackPos.Sub(startAttackPos)

		startEntityPos := entity.PrevPosition
		endEntityPos := entity.Position
		entityPosDelta := endEntityPos.Sub(startEntityPos)

		totalDist, count := float32(0), float32(0)
		minDist := float32(math32.MaxFloat32 - 1)
		for partialTicks := float32(0); partialTicks <= 1; partialTicks += interpolationIncrement {
			currentAttackPos := startAttackPos.Add(attackPosDelta.Mul(partialTicks))
			currentAttackDirection := startRotation.Add(attackRotationDelta.Mul(partialTicks))
			currentEntityPos := startEntityPos.Add(entityPosDelta.Mul(partialTicks))

			// Calculate the attack direction vector.
			directionVector := game.DirectionVector(currentAttackDirection.Z(), currentAttackDirection.X())
			entityBB := entity.Box(currentEntityPos).Grow(0.1)

			closestPoint := game.ClosestPointToBBoxDirectional(
				currentAttackPos,
				startAttackDirection,
				endAttackDirection,
				entityBB,
				14.0,
			)
			closestDistance := closestPoint.Sub(currentAttackPos).Len()
			if closestDistance < minDist {
				minDist = closestDistance
			}

			rayResult, hit := trace.BBoxIntercept(entityBB, currentAttackPos, currentAttackPos.Add(directionVector.Mul(14.0)))
			if !hit {
				continue
			}

			distance := rayResult.Position().Sub(currentAttackPos).Len()
			totalDist += distance
			count++
		}

		// TODO: Hitbox detection.
		if count == 0 {
			return true
		}

		// Check if the mininum distance from the player's attack position is an amount near three blocks
		// and if the average distance is greater than three blocks.
		avgDist := totalDist / count
		if minDist >= 2.95 && avgDist > 3 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("distance", avgDist)
			data.Set("origin_min", minDist)
			d.Fail(p, data)
			return true
		}

		d.Debuff(0.004)
	}

	return true
}
