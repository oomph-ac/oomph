package component

import (
	"time"

	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	COMBAT_LERP_POSITION_STEPS = 10

	COMBAT_SURVIVAL_ENTITY_SEARCH_RADIUS float32 = 6.0
	COMBAT_SURVIVAL_REACH                float32 = 2.9
)

// AuthoritativeCombatComponent is responsible for managing and simulating combat mechanics for players on the server.
// It ensures that all players operate under the same rules and conditions during combat, and determines wether or not an attack can be sent to the server.
type AuthoritativeCombatComponent struct {
	mPlayer *player.Player

	entityBB                                      cube.BBox
	startAttackPos, startEntityPos, startRotation mgl32.Vec3
	endAttackPos, endEntityPos, endRotation       mgl32.Vec3
	targetedEntity                                *entity.Entity

	swingTick int64

	// raycastResults are the raycasted distances for the combat validation done.
	raycastResults []float32
	// rawResults are the raw closest distance from the entity BB to the attack position for the
	// combat validation done.
	rawResults []float32
	// hooks are the combat hooks that utilize the results of this combat component.
	hooks []player.CombatHook

	// attackInput is the input the client sent to attack an entity.
	attackInput *packet.InventoryTransaction
	// checkMisprediction is true if the client swings in the air and the combat component is not ACK dependent.
	checkMisprediction bool
	// ackDependent is true if the combat component should rely on client-ACK'ed entities to verify combat.
	ackDependent bool
}

func NewAuthoritativeCombatComponent(p *player.Player, ackDependent bool) *AuthoritativeCombatComponent {
	return &AuthoritativeCombatComponent{
		mPlayer:      p,
		ackDependent: ackDependent,

		raycastResults: make([]float32, 0, COMBAT_LERP_POSITION_STEPS*2),
		rawResults:     make([]float32, 0, COMBAT_LERP_POSITION_STEPS),
		hooks:          []player.CombatHook{},
	}
}

// Hook adds a hook to the combat component so it may utilize the results of this combat component.
func (c *AuthoritativeCombatComponent) Hook(h player.CombatHook) {
	c.hooks = append(c.hooks, h)
}

// Attack notifies the combat component of an attack.
func (c *AuthoritativeCombatComponent) Attack(input *packet.InventoryTransaction) {
	// Do not try to allow another hit if the member player has already notified us of an attack this tick.
	if c.attackInput != nil {
		return
	}

	if input == nil {
		//assert.IsTrue(!c.ackDependent, "ack-dependent combat component is should not calculate misprediction")
		if c.ackDependent {
			c.mPlayer.Disconnect(game.ErrorInternalUnexpectedNullInput)
			return
		}

		c.checkMisprediction = true
		return
	}

	data := input.TransactionData.(*protocol.UseItemOnEntityTransactionData)
	if c.ackDependent {
		e := c.mPlayer.ClientEntityTracker().FindEntity(data.TargetEntityRuntimeID)
		if e == nil {
			return
		}
		c.targetedEntity = e

		c.startAttackPos = c.mPlayer.Movement().Client().LastPos()
		c.endAttackPos = c.mPlayer.Movement().Client().Pos()

		c.startEntityPos = e.PrevPosition
		c.endEntityPos = e.Position
		c.entityBB = e.Box(mgl32.Vec3{})
	} else {
		e := c.mPlayer.EntityTracker().FindEntity(data.TargetEntityRuntimeID)
		if e == nil {
			return
		}
		c.targetedEntity = e

		rewindPos, ok := e.Rewind(c.mPlayer.ClientTick)
		if !ok {
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "no rewind positions available for entity %d", data.TargetEntityRuntimeID)
			return
		}

		c.startAttackPos = c.mPlayer.Movement().LastPos()
		c.endAttackPos = c.mPlayer.Movement().Pos()

		c.startEntityPos = rewindPos.PrevPosition
		c.endEntityPos = rewindPos.Position
		c.entityBB = e.Box(mgl32.Vec3{})
	}

	if c.mPlayer.Movement().Sneaking() {
		c.startAttackPos[1] += 1.54
		c.endAttackPos[1] += 1.54
	} else {
		c.startAttackPos[1] += 1.62
		c.endAttackPos[1] += 1.62
	}

	c.attackInput = input
}

func (c *AuthoritativeCombatComponent) Calculate() bool {
	// There is no attack input for this tick.
	if !c.checkMisprediction && c.attackInput == nil {
		return false
	}

	if gamemode := c.mPlayer.GameMode; gamemode != packet.GameTypeSurvival && gamemode != packet.GameTypeAdventure {
		return false
	}

	t := time.Now()
	defer func() {
		delta := float64(time.Since(t).Nanoseconds()) / 1_000_000.0
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "took %fms to process [ackDependent=%v]", delta, c.ackDependent)
	}()

	var (
		closestHitResult trace.BBoxResult
		lerpedAtClosest  lerpedResult

		closestRaycastDist float32 = 1_000_000
		closestRawDist     float32 = 1_000_000
	)

	if c.checkMisprediction {
		if c.mPlayer.LastEquipmentData == nil || !c.checkForMispredictedEntity() {
			c.reset()
			return false
		}
	}

	movement := c.mPlayer.Movement()
	hasNonSmoothTeleport := movement.HasTeleport() && !movement.TeleportSmoothed()
	if hasNonSmoothTeleport {
		c.startAttackPos = movement.TeleportPos()
		c.endAttackPos = movement.TeleportPos()
	}

	c.startRotation = c.mPlayer.Movement().LastRotation()
	c.endRotation = c.mPlayer.Movement().Rotation()

	var (
		altStartEntityPos mgl32.Vec3
		altEndEntityPos   mgl32.Vec3
		altPosDelta       mgl32.Vec3
	)

	if c.ackDependent {
		altStartEntityPos = c.targetedEntity.PrevPosition
		altEndEntityPos = c.targetedEntity.Position
		altPosDelta = altEndEntityPos.Sub(altStartEntityPos)
	}

	hitValid := false
	stepAmt := 1.0 / float32(COMBAT_LERP_POSITION_STEPS)
	for partialTicks := float32(0.0); partialTicks <= 1; partialTicks += stepAmt {
		lerpedResult := c.lerp(partialTicks)
		entityBB := c.entityBB.Translate(lerpedResult.entityPos).Grow(0.1)
		dV := game.DirectionVector(lerpedResult.rotation.Z(), lerpedResult.rotation.X())

		if c.mPlayer.InputMode != packet.InputModeTouch {
			if entityBB.Vec3Within(lerpedResult.attackPos) {
				closestRaycastDist = 0
				hitValid = true
				c.raycastResults = append(c.raycastResults, 0.0)
				continue
			}

			if hitResult, ok := trace.BBoxIntercept(entityBB, lerpedResult.attackPos, lerpedResult.attackPos.Add(dV.Mul(7.0))); ok {
				raycastDist := lerpedResult.attackPos.Sub(hitResult.Position()).Len()
				hitValid = hitValid || raycastDist <= COMBAT_SURVIVAL_REACH
				c.raycastResults = append(c.raycastResults, raycastDist)

				if raycastDist < closestRaycastDist {
					closestRaycastDist = raycastDist
					closestHitResult = hitResult
					lerpedAtClosest = lerpedResult
				}
			}

			// Check a possible alternative position for a race condition where minecraft is casting a ray but the
			// entity has already been ticked. This should only be done if the combat component is acknowledgment dependent.
			if c.ackDependent {
				if c.ackDependent && hasNonSmoothTeleport {
					c.startAttackPos = c.mPlayer.Movement().Client().LastPos()
				}

				altLerpedPos := altStartEntityPos.Add(altPosDelta)
				altEntityBB := c.entityBB.Translate(altLerpedPos).Grow(0.1)

				if altEntityBB.Vec3Within(lerpedResult.attackPos) {
					closestRaycastDist = 0
					hitValid = true
					c.raycastResults = append(c.raycastResults, 0.0)
					continue
				}

				if hitResult, ok := trace.BBoxIntercept(altEntityBB, lerpedResult.attackPos, lerpedResult.attackPos.Add(dV.Mul(7.0))); ok {
					raycastDist := lerpedResult.attackPos.Sub(hitResult.Position()).Len()
					hitValid = hitValid || raycastDist <= COMBAT_SURVIVAL_REACH
					c.raycastResults = append(c.raycastResults, raycastDist)

					if raycastDist < closestRaycastDist {
						closestRaycastDist = raycastDist
						closestHitResult = hitResult
						lerpedAtClosest = lerpedResult
					}
				}
			}
		}

		rawDist := lerpedResult.attackPos.Sub(game.ClosestPointToBBox(lerpedResult.attackPos, entityBB.Grow(0.1))).Len()
		c.rawResults = append(c.rawResults, rawDist)
		if rawDist < closestRawDist {
			closestRawDist = rawDist
		}

		if c.mPlayer.InputMode == packet.InputModeTouch {
			hitValid = hitValid || rawDist <= COMBAT_SURVIVAL_REACH
		}
	}

	// If the hit is valid and the player is not on touch mode, check if the closest calculated ray from the player's eye position to the bounding box
	// of the entity, has any intersecting blocks. If there are blocks that are in the way of the ray then the hit is invalid.
	if !c.ackDependent && hitValid && c.mPlayer.InputMode != packet.InputModeTouch && closestRaycastDist > 0 {
		start, end := lerpedAtClosest.attackPos, closestHitResult.Position()

	check_blocks_between_ray:
		for _, blockPos := range game.BlocksBetween(start, end) {
			flooredBlockPos := cube.PosFromVec3(blockPos)
			blockInWay := c.mPlayer.World.Block(df_cube.Pos(flooredBlockPos))
			if utils.IsBlockPassInteraction(blockInWay) {
				continue
			}

			// Iterate through each block's bounding boxes and check if it is in the way of the ray.
			for _, blockBB := range utils.BlockBoxes(blockInWay, flooredBlockPos, c.mPlayer.World) {
				blockBB = blockBB.Translate(blockPos)
				if _, ok := trace.BBoxIntercept(blockBB, start, end); ok {
					hitValid = false
					c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "hit was invalidated due to %s blocking attack ray", utils.BlockName(blockInWay))
					break check_blocks_between_ray
				}
			}
		}
	}
	c.mPlayer.Dbg.Notify(player.DebugModeCombat, !hitValid && !c.checkMisprediction, "<red>hit was invalidated due to distance check</red> [ackDependent=%v] (raycast=%f, raw=%f)", c.ackDependent, closestRaycastDist, closestRawDist)

	// If this is the full-authoritative combat component, and the hit is valid, send the attack packet to the server.
	if hitValid {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "<green>hit sent to the server</green> (raycast=%f raw=%f)", closestRaycastDist, closestRawDist)
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, c.checkMisprediction, "<yellow>client mispredicted air hit, but actually attacked entity</yellow>")
		if !c.ackDependent {
			c.mPlayer.SendPacketToServer(c.attackInput)
		}
	}

	for _, hook := range c.hooks {
		hook(c)
	}
	c.reset()

	return true
}

func (c *AuthoritativeCombatComponent) Swing() {
	c.swingTick = int64(c.mPlayer.SimulationFrame)
}

func (c *AuthoritativeCombatComponent) LastSwing() int64 {
	return c.swingTick
}

func (c *AuthoritativeCombatComponent) Raycasts() []float32 {
	return c.raycastResults
}

func (c *AuthoritativeCombatComponent) Raws() []float32 {
	return c.rawResults
}

// checkForMispredictedEntity returns true if an entity were found to be in the way of the combat component.
func (c *AuthoritativeCombatComponent) checkForMispredictedEntity() bool {
	var (
		minDist        float32 = 1_000_000
		targetedEntity *entity.Entity
		rewindData     entity.HistoricalPosition
		eid            uint64
	)

	c.startAttackPos = c.mPlayer.Movement().LastPos()
	c.endAttackPos = c.mPlayer.Movement().Pos()
	if c.mPlayer.Movement().Sneaking() {
		c.startAttackPos[1] += 1.54
		c.endAttackPos[1] += 1.54
	} else {
		c.startAttackPos[1] += 1.62
		c.endAttackPos[1] += 1.62
	}

	for rid, e := range c.mPlayer.EntityTracker().All() {
		rewind, ok := e.Rewind(c.mPlayer.ClientTick)
		if !ok {
			continue
		}

		dist := rewind.Position.Sub(c.endAttackPos).Len()
		if dist <= COMBAT_SURVIVAL_ENTITY_SEARCH_RADIUS {
			if dist < minDist {
				minDist = dist
				rewindData = rewind
				targetedEntity = e
				eid = rid
			}
		}
	}

	if targetedEntity == nil {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "no mispredicted entity found")
		return false
	}

	c.startEntityPos = rewindData.PrevPosition
	c.endEntityPos = rewindData.Position
	c.entityBB = targetedEntity.Box(mgl32.Vec3{})
	c.attackInput = &packet.InventoryTransaction{
		TransactionData: &protocol.UseItemOnEntityTransactionData{
			TargetEntityRuntimeID: eid,
			ActionType:            protocol.UseItemOnEntityActionAttack,
			HotBarSlot:            int32(c.mPlayer.LastEquipmentData.HotBarSlot),
			HeldItem:              c.mPlayer.LastEquipmentData.NewItem,
			Position:              c.endAttackPos,
			ClickedPosition:       mgl32.Vec3{},
		},
	}
	return true
}

func (c *AuthoritativeCombatComponent) reset() {
	c.attackInput = nil
	c.targetedEntity = nil
	c.checkMisprediction = false
	c.raycastResults = c.raycastResults[:0]
	c.rawResults = c.rawResults[:0]
}

type lerpedResult struct {
	attackPos mgl32.Vec3
	entityPos mgl32.Vec3
	rotation  mgl32.Vec3
}

func (c *AuthoritativeCombatComponent) lerp(partialTicks float32) lerpedResult {
	if partialTicks == 0.0 {
		return lerpedResult{
			attackPos: c.startAttackPos,
			entityPos: c.startEntityPos,
			rotation:  c.startRotation,
		}
	} else if partialTicks == 1.0 {
		return lerpedResult{
			attackPos: c.endAttackPos,
			entityPos: c.endEntityPos,
			rotation:  c.endRotation,
		}
	}

	attackPosDelta := c.endAttackPos.Sub(c.startAttackPos).Mul(partialTicks)
	entPosDelta := c.endEntityPos.Sub(c.startEntityPos).Mul(partialTicks)
	rotationDelta := c.endRotation.Sub(c.startRotation).Mul(partialTicks)

	return lerpedResult{
		attackPos: c.startAttackPos.Add(attackPosDelta),
		entityPos: c.startEntityPos.Add(entPosDelta),
		rotation:  c.startRotation.Add(rotationDelta),
	}
}
