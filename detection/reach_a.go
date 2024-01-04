package detection

import (
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDReachA = "reach_a"

const interpolationIncrement float32 = 1 / 20.0
const noHit float32 = 69.0

type ReachA struct {
	BaseDetection

	run            bool
	targetedEntity uint64

	startAttackPos mgl32.Vec3
}

func (d *ReachA) ID() string {
	return DetectionIDReachA
}

func (d *ReachA) Name() (string, string) {
	return "Reach", "A"
}

func (d *ReachA) Description() string {
	return "Detects when a player's attack range exceeds 3 blocks."
}

func (d *ReachA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.CombatMode != player.AuthorityModeSemi {
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

		entity := p.Handler(player.HandlerIDEntities).(*player.EntityHandler).FindEntity(dat.TargetEntityRuntimeID)
		if entity == nil {
			return true
		}

		d.targetedEntity = dat.TargetEntityRuntimeID
		d.startAttackPos = p.Handler(player.HandlerIDMovement).(*player.MovementHandler).PrevClientPosition
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

		entity := p.Handler(player.HandlerIDEntities).(*player.EntityHandler).FindEntity(d.targetedEntity)
		if entity == nil {
			return true
		}

		movementHandler := p.Handler(player.HandlerIDMovement).(*player.MovementHandler)
		offset := float32(1.62)
		if movementHandler.Sneaking {
			offset = 1.54
		}

		startDirection := movementHandler.PrevRotation
		endDirection := movementHandler.Rotation
		attackDirectionDelta := endDirection.Sub(startDirection)

		// Do not attempt a raycast if there is a significant change in the player's yaw.
		if attackDirectionDelta.Z() >= 20 {
			p.Message("yaw change too high")
			return true
		}

		startAttackPos := d.startAttackPos.Add(mgl32.Vec3{0, offset})
		endAttackPos := startAttackPos.Add(movementHandler.PrevClientVel).Sub(movementHandler.Knockback)
		attackPosDelta := endAttackPos.Sub(startAttackPos)

		startEntityPos := entity.PrevPosition
		endEntityPos := entity.PrevPosition.Add(entity.PrevVelocity)
		// TODO: See if we need to actually interpolate the entity's position.
		entityPosDelta := endEntityPos.Sub(startEntityPos)

		minDist := noHit
		for partialTicks := float32(0); partialTicks <= 1; partialTicks += interpolationIncrement {
			currentAttackPos := startAttackPos.Add(attackPosDelta.Mul(partialTicks))
			currentAttackDirection := startDirection.Add(attackDirectionDelta.Mul(partialTicks))
			currentEntityPos := startEntityPos.Add(entityPosDelta.Mul(partialTicks))

			// Calculate the attack direction vector.
			directionVector := game.DirectionVector(currentAttackDirection.Z(), currentAttackDirection.X())
			entityBB := entity.Box(currentEntityPos).Grow(0.1)

			rayResult, hit := trace.BBoxIntercept(entityBB, currentAttackPos, currentAttackPos.Add(directionVector.Mul(14.0)))
			if !hit {
				continue
			}

			distance := rayResult.Position().Sub(currentAttackPos).Len()
			if distance < minDist {
				minDist = distance
			}
		}

		// TODO: Hitbox detection.
		if minDist == noHit {
			return true
		}

		// TODO: Handle gamemode (don't detect in Creative, Spectator, etc.)
		if minDist > 3.005 {
			d.Fail(p, 1.001, Data{"distance", game.Round32(minDist, 2)})
			return true
		}
		d.Debuff(0.001)
	}

	return true
}
