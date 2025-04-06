package component

import (
	"time"

	"github.com/chewxy/math32"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oconfig"
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

func init() {
	// There is nothing to say.
}

// AuthoritativeCombatComponent is responsible for managing and simulating combat mechanics for players on the server.
// It ensures that all players operate under the same rules and conditions during combat, and determines whether an attack can be sent to the server.
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
}

func NewAuthoritativeCombatComponent(p *player.Player) *AuthoritativeCombatComponent {
	return &AuthoritativeCombatComponent{
		mPlayer:        p,
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
		if oconfig.Combat().FullAuthoritative {
			c.checkMisprediction = true
		}
		return
	}

	data := input.TransactionData.(*protocol.UseItemOnEntityTransactionData)
	e := c.mPlayer.EntityTracker().FindEntity(data.TargetEntityRuntimeID)
	if e == nil {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "entity %d not found", data.TargetEntityRuntimeID)
		return
	}
	c.targetedEntity = e

	if oconfig.Combat().FullAuthoritative {
		rewindPos, ok := e.Rewind(c.mPlayer.ClientTick)
		if !ok {
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "no rewind positions available for entity %d", data.TargetEntityRuntimeID)
			return
		}
		c.startEntityPos = rewindPos.PrevPosition
		c.endEntityPos = rewindPos.Position
	} else {
		c.startEntityPos = e.PrevPosition
		c.endEntityPos = e.Position
	}

	c.startAttackPos = c.mPlayer.Movement().LastPos()
	c.endAttackPos = c.mPlayer.Movement().Pos()
	c.entityBB = e.Box(mgl32.Vec3{})

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
	defer c.reset()
	if !c.checkMisprediction && c.attackInput == nil {
		return false
	}

	// Allow any hits if the player is in the correct gamemode.
	if gamemode := c.mPlayer.GameMode; gamemode != packet.GameTypeSurvival && gamemode != packet.GameTypeAdventure {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "player is in gamemode %d, allowing hit", gamemode)
		if c.attackInput != nil {
			c.mPlayer.SendPacketToServer(c.attackInput)
		}
		return true
	}

	t := time.Now()
	defer func() {
		delta := float64(time.Since(t).Nanoseconds()) / 1_000_000.0
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "took %.4fms to process", delta)
	}()

	var (
		closestHitResult trace.BBoxResult
		lerpedAtClosest  lerpedResult

		closestRaycastDist float32 = 1_000_000
		closestRawDist     float32 = 1_000_000
		closestAngle       float32 = 1_000_000

		raycastHit bool
	)

	if c.checkMisprediction {
		if c.mPlayer.LastEquipmentData == nil || !c.checkForMispredictedEntity() {
			return false
		}
	}

	movement := c.mPlayer.Movement()
	if movement.PendingCorrections() > 0 && !oconfig.Combat().FullAuthoritative {
		return false
	}

	hasNonSmoothTeleport := movement.HasTeleport() && !movement.TeleportSmoothed()
	if hasNonSmoothTeleport {
		c.startAttackPos = movement.TeleportPos()
		c.endAttackPos = movement.TeleportPos()
	}

	c.startRotation = c.mPlayer.Movement().LastRotation()
	c.endRotation = c.mPlayer.Movement().Rotation()

	var (
		altEntityStartPos mgl32.Vec3
		altEntityEndPos   mgl32.Vec3
		altEntityPosDelta mgl32.Vec3
	)
	if !oconfig.Combat().FullAuthoritative {
		altEntityStartPos = c.targetedEntity.PrevPosition
		altEntityEndPos = c.targetedEntity.Position
		altEntityPosDelta = altEntityEndPos.Sub(altEntityStartPos)
	}

	hitValid := false
	stepAmt := 1.0 / float32(COMBAT_LERP_POSITION_STEPS)
	for partialTicks := float32(0.0); partialTicks <= 1; partialTicks += stepAmt {
		lerpedResult := c.lerp(partialTicks)
		entityBB := c.entityBB.Translate(lerpedResult.entityPos).Grow(0.1)
		dV := game.DirectionVector(lerpedResult.rotation.Z(), lerpedResult.rotation.X())

		// If the attack position is within the entity's bounding box, the hit is valid and we don't have to do any further checks.
		if entityBB.Vec3Within(lerpedResult.attackPos) {
			closestRaycastDist = 0
			closestRawDist = 0
			closestAngle = 0
			hitValid = true
			c.raycastResults = append(c.raycastResults, 0.0)
			c.rawResults = append(c.rawResults, 0.0)
			break
		}

		if angle := math32.Abs(game.AngleToPoint(lerpedResult.attackPos, lerpedResult.entityPos, lerpedResult.rotation)[0]); angle < closestAngle {
			closestAngle = angle
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
			raycastHit = true
		}

		// An extra raycast is ran here with the current entity position, as the client may have ticked
		// the entity to a new position while the frame logic was running (where attacks are done). This is only
		// required if FullAuthoritative is *disabled*, as we want to match the client's own logic as closely as possible.
		if !oconfig.Combat().FullAuthoritative {
			altEntPos := altEntityStartPos.Add(altEntityPosDelta.Mul(partialTicks))
			altEntityBB := c.entityBB.Translate(altEntPos).Grow(0.1)
			if hitResult, ok := trace.BBoxIntercept(altEntityBB, lerpedResult.attackPos, lerpedResult.attackPos.Add(dV.Mul(7.0))); ok {
				altRaycastDist := lerpedResult.attackPos.Sub(hitResult.Position()).Len()
				hitValid = hitValid || altRaycastDist <= COMBAT_SURVIVAL_REACH
				c.raycastResults = append(c.raycastResults, altRaycastDist)
				if altRaycastDist < closestRaycastDist {
					closestRaycastDist = altRaycastDist
					closestHitResult = hitResult
					lerpedAtClosest = lerpedResult
				}
				raycastHit = true
			}
		}

		rawDist := lerpedResult.attackPos.Sub(game.ClosestPointToBBox(lerpedResult.attackPos, entityBB.Grow(0.1))).Len()
		c.rawResults = append(c.rawResults, rawDist)
		if rawDist < closestRawDist {
			closestRawDist = rawDist
		}

		/* if c.mPlayer.InputMode == packet.InputModeTouch {
			hitValid = hitValid || rawDist <= COMBAT_SURVIVAL_REACH
		} */
	}

	// Only allow the raw distance check to be use for touch players if a raycast is unable to land on the entity. This prevents clients
	// abusing spoofing their input to gain a slight reach advantage. We also want to make sure we're not allowing the player to hit entities
	// that are behind them. 110 degrees is MC:BE's maximum camera FOV.
	if !hitValid && c.mPlayer.InputMode == packet.InputModeTouch {
		hitValid = closestRawDist <= COMBAT_SURVIVAL_REACH && closestAngle <= 110.0
	}

	// If the hit is valid and the player is not on touch mode, check if the closest calculated ray from the player's eye position to the bounding box
	// of the entity, has any intersecting blocks. If there are blocks that are in the way of the ray then the hit is invalid.
	if hitValid && raycastHit && closestRaycastDist > 0 {
		start, end := lerpedAtClosest.attackPos, closestHitResult.Position()

	check_blocks_between_ray:
		for blockPos := range game.BlocksBetween(start, end) {
			flooredBlockPos := cube.PosFromVec3(blockPos)
			blockInWay := c.mPlayer.WorldTx().Block(df_cube.Pos(flooredBlockPos))
			if utils.IsBlockPassInteraction(blockInWay) {
				continue
			}

			// Iterate through each block's bounding boxes and check if it is in the way of the ray.
			for _, blockBB := range utils.BlockBoxes(blockInWay, flooredBlockPos, c.mPlayer.WorldTx()) {
				blockBB = blockBB.Translate(blockPos)
				if _, ok := trace.BBoxIntercept(blockBB, start, end); ok {
					hitValid = false
					c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "hit was invalidated due to %s blocking attack ray", utils.BlockName(blockInWay))
					break check_blocks_between_ray
				}
			}
		}
	}

	c.mPlayer.Dbg.Notify(
		player.DebugModeCombat,
		!hitValid && !c.checkMisprediction,
		"<red>hit was invalidated due to distance check</red> (raycast=%f, raw=%f)",
		closestRaycastDist,
		closestRawDist,
	)

	// If this is the full-authoritative combat component, and the hit is valid, send the attack packet to the server.
	if hitValid {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "<green>hit sent to the server</green> (raycast=%f raw=%f)", closestRaycastDist, closestRawDist)
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, c.checkMisprediction, "<yellow>client mispredicted air hit, but actually attacked entity</yellow>")
		c.mPlayer.SendPacketToServer(c.attackInput)
	}

	for _, hook := range c.hooks {
		hook(c)
	}
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
