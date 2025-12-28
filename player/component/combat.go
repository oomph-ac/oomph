package component

import (
	"time"

	"github.com/chewxy/math32"
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
	CombatLerpPositionSteps = 10

	CombatSurvivalEntitySearchRadius float32 = 6.0
	CombatSurvivalReach              float32 = 2.9
)

func init() {
	// There is nothing to say.
}

// AuthoritativeCombatComponent is responsible for managing and simulating combat mechanics for players on the server.
// It ensures that all players operate under the same rules and conditions during combat, and determines whether an attack can be sent to the server.
type AuthoritativeCombatComponent struct {
	mPlayer *player.Player

	startAttackPos, startEntityPos, startRotation mgl32.Vec3
	endAttackPos, endEntityPos, endRotation       mgl32.Vec3

	targetedEntity         *entity.Entity
	targetedRuntimeID      uint64
	entityBB               cube.BBox
	uniqueAttackedEntities map[uint64]*entity.Entity

	swingTick int64

	// raycastResults are the raycasted distances for the combat validation done.
	raycastResults []float32
	// rawResults are the raw closest distance from the entity BB to the attack position for the
	// combat validation done.
	rawResults []float32
	// angleResults are the angles between the attack position and the entity position for the combat validation done.
	angleResults []float32
	// hooks are the combat hooks that utilize the results of this combat component.
	hooks []player.CombatHook

	// attackInput is the input the client sent to attack an entity.
	attackInput *packet.InventoryTransaction
	// checkMisprediction is true if the client swings in the air and the combat component is not ACK dependent.
	checkMisprediction bool

	attacked         bool
	useClientTracker bool
}

func NewAuthoritativeCombatComponent(p *player.Player, useClientTracker bool) *AuthoritativeCombatComponent {
	return &AuthoritativeCombatComponent{
		mPlayer:                p,
		raycastResults:         make([]float32, 0, CombatLerpPositionSteps*2),
		rawResults:             make([]float32, 0, CombatLerpPositionSteps*2),
		angleResults:           make([]float32, 0, CombatLerpPositionSteps*2),
		hooks:                  []player.CombatHook{},
		uniqueAttackedEntities: make(map[uint64]*entity.Entity),

		useClientTracker: useClientTracker,
	}
}

// Hook adds a hook to the combat component so it may utilize the results of this combat component.
func (c *AuthoritativeCombatComponent) Hook(h player.CombatHook) {
	c.hooks = append(c.hooks, h)
}

func (c *AuthoritativeCombatComponent) UniqueAttacks() map[uint64]*entity.Entity {
	return c.uniqueAttackedEntities
}

// Attack notifies the combat component of an attack.
func (c *AuthoritativeCombatComponent) Attack(input *packet.InventoryTransaction) {
	var (
		data *protocol.UseItemOnEntityTransactionData
		e    *entity.Entity
	)
	if input != nil {
		data = input.TransactionData.(*protocol.UseItemOnEntityTransactionData)
		e = c.entityTracker().FindEntity(data.TargetEntityRuntimeID)
		if e == nil {
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "entity %d not found", data.TargetEntityRuntimeID)
			return
		}
		c.uniqueAttackedEntities[data.TargetEntityRuntimeID] = e
		c.checkMisprediction = false
	}

	// Do not try to allow another hit if the member player has already notified us of an attack this tick.
	if c.attackInput != nil {
		return
	}
	if input == nil {
		// We should only check for mispredictions if we are utilizing the server-state entity tracker.
		if !c.useClientTracker {
			c.checkMisprediction = true
			c.attacked = true
			c.startAttackPos = c.mPlayer.Movement().LastPos()
			c.endAttackPos = c.mPlayer.Movement().Pos()
			if c.mPlayer.Movement().Sneaking() {
				c.startAttackPos[1] += game.SneakingPlayerHeightOffset
				c.endAttackPos[1] += game.SneakingPlayerHeightOffset
			} else {
				c.startAttackPos[1] += game.DefaultPlayerHeightOffset
				c.endAttackPos[1] += game.DefaultPlayerHeightOffset
			}
		}
		return
	}

	// The reach/hitbox detection should only be applied to other players.
	if c.useClientTracker && !e.IsPlayer {
		return
	}

	c.attacked = true
	c.targetedEntity = e
	c.targetedRuntimeID = data.TargetEntityRuntimeID

	if !c.useClientTracker {
		// Use the server-side rewound state of the entity, opposed to the fully compensated client-sided view.
		rewindPos, ok := e.Rewind(c.mPlayer.ClientTick)
		if !ok {
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "no rewind positions available for entity %d", data.TargetEntityRuntimeID)
			return
		}
		c.startEntityPos = rewindPos.PrevPosition
		c.endEntityPos = rewindPos.Position
		c.startAttackPos = c.mPlayer.Movement().LastPos()
		c.endAttackPos = c.mPlayer.Movement().Pos()
	} else {
		c.startEntityPos = e.PrevPosition
		c.endEntityPos = e.Position
		c.startAttackPos = c.mPlayer.Movement().Client().LastPos()
		c.endAttackPos = c.mPlayer.Movement().Client().Pos()
	}
	c.entityBB = e.Box(mgl32.Vec3{})

	if c.mPlayer.Movement().Sneaking() {
		c.startAttackPos[1] += game.SneakingPlayerHeightOffset
		c.endAttackPos[1] += game.SneakingPlayerHeightOffset
	} else {
		c.startAttackPos[1] += game.DefaultPlayerHeightOffset
		c.endAttackPos[1] += game.DefaultPlayerHeightOffset
	}
	c.attackInput = input
}

func (c *AuthoritativeCombatComponent) Calculate() bool {
	if !c.attacked {
		return false
	}

	// There is no attack input for this tick.
	defer c.Reset()

	if !c.checkMisprediction && c.attackInput == nil {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "no attack input for this tick, skipping combat calculation")
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
		closestAngle       float32 = 1_000_000

		closestRawDist float32 = 1_000_000
		closestRawPos  mgl32.Vec3

		raycastHit bool
	)

	if c.checkMisprediction {
		/*if c.mPlayer.LastEquipmentData == nil {
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "no last equipment data available, cannot check for mispredicted entity")
			return false
		} else */
		if !c.checkForMispredictedEntity() {
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "no mispredicted entity found, skipping combat calculation")
			return false
		}
	}

	movement := c.mPlayer.Movement()
	if movement.PendingCorrections() > 0 && c.useClientTracker {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "movement component indicates pending corrections (%d) - hit invalidated", movement.PendingCorrections())
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
	if c.useClientTracker {
		altEntityStartPos = c.targetedEntity.PrevPosition
		altEntityEndPos = c.targetedEntity.Position
		altEntityPosDelta = altEntityEndPos.Sub(altEntityStartPos)
	}

	hitValid := false
	stepAmt := 1.0 / float32(CombatLerpPositionSteps)
	for partialTicks := float32(0.0); partialTicks <= 1; partialTicks += stepAmt {
		lerpedResult := c.lerp(partialTicks)
		entityBB := c.entityBB.Translate(lerpedResult.entityPos).Grow(0.1)
		dV := game.DirectionVector(lerpedResult.rotation.Z(), lerpedResult.rotation.X())

		// If the attack position is within the entity's bounding box, the hit is valid and we don't have to do any further checks.
		if entityBB.Vec3Within(lerpedResult.attackPos) {
			closestRaycastDist = 0
			closestRawDist = 0
			closestRawPos = lerpedResult.attackPos
			closestAngle = 0
			hitValid = true
			c.raycastResults = append(c.raycastResults, 0.0)
			c.rawResults = append(c.rawResults, 0.0)
			break
		}

		angle := math32.Abs(game.AngleToPoint(lerpedResult.attackPos, lerpedResult.entityPos, lerpedResult.rotation)[0])
		c.angleResults = append(c.angleResults, angle)
		if angle < closestAngle {
			closestAngle = angle
		}

		if hitResult, ok := trace.BBoxIntercept(entityBB, lerpedResult.attackPos, lerpedResult.attackPos.Add(dV.Mul(7.0))); ok {
			raycastDist := lerpedResult.attackPos.Sub(hitResult.Position()).Len()
			hitValid = hitValid || raycastDist <= CombatSurvivalReach
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
		if c.useClientTracker {
			altEntPos := altEntityStartPos.Add(altEntityPosDelta.Mul(partialTicks))
			altEntityBB := c.entityBB.Translate(altEntPos).Grow(0.1)

			altAngle := math32.Abs(game.AngleToPoint(lerpedResult.attackPos, altEntPos, lerpedResult.rotation)[0])
			c.angleResults = append(c.angleResults, altAngle)
			if altAngle < closestAngle {
				closestAngle = altAngle
			}

			if hitResult, ok := trace.BBoxIntercept(altEntityBB, lerpedResult.attackPos, lerpedResult.attackPos.Add(dV.Mul(7.0))); ok {
				altRaycastDist := lerpedResult.attackPos.Sub(hitResult.Position()).Len()
				hitValid = hitValid || altRaycastDist <= CombatSurvivalReach
				c.raycastResults = append(c.raycastResults, altRaycastDist)
				if altRaycastDist < closestRaycastDist {
					closestRaycastDist = altRaycastDist
					closestHitResult = hitResult
					lerpedAtClosest = lerpedResult
				}
				raycastHit = true
			}
			closestPoint := game.ClosestPointToBBox(lerpedResult.attackPos, altEntityBB)
			rawDist := lerpedResult.attackPos.Sub(closestPoint).Len()
			c.rawResults = append(c.rawResults, rawDist)
			if rawDist < closestRawDist {
				closestRawDist = rawDist
				closestRawPos = closestPoint
			}
		}

		closestPoint := game.ClosestPointToBBox(lerpedResult.attackPos, entityBB)
		rawDist := lerpedResult.attackPos.Sub(closestPoint).Len()
		c.rawResults = append(c.rawResults, rawDist)
		if rawDist < closestRawDist {
			closestRawDist = rawDist
			closestRawPos = closestPoint
		}

		/* if c.mPlayer.InputMode == packet.InputModeTouch {
			hitValid = hitValid || rawDist <= COMBAT_SURVIVAL_REACH
		} */
	}

	// Only allow the raw distance check to be use for touch players if a raycast is unable to land on the entity. This prevents clients
	// abusing spoofing their input to gain a slight reach advantage. We also want to make sure we're not allowing the player to hit entities
	// that are behind them.
	if !hitValid && c.mPlayer.InputMode == packet.InputModeTouch {
		hitValid = closestRawDist <= CombatSurvivalReach && closestAngle <= c.mPlayer.Opts().Combat.MaximumAttackAngle
		if hitValid {
			lerpedAtClosest.attackPos = c.endAttackPos
			lerpedAtClosest.entityPos = c.startEntityPos
			lerpedAtClosest.rotation = c.endRotation
			utils.ModifyBBoxResult(&closestHitResult, c.entityBB, closestRawPos, 0)
			raycastHit = true
		}
	}

	// If the hit is valid and the player is not on touch mode, check if the closest calculated ray from the player's eye position to the bounding box
	// of the entity, has any intersecting blocks. If there are blocks that are in the way of the ray then the hit is invalid.
	if !c.useClientTracker && hitValid && raycastHit && closestRaycastDist > 0 {
		start, end := lerpedAtClosest.attackPos, closestHitResult.Position()
	check_blocks_between_ray:
		for blockPos := range game.BlocksBetween(start, end, 50) {
			flooredBlockPos := cube.PosFromVec3(blockPos)
			blockInWay := c.mPlayer.World().Block(df_cube.Pos(flooredBlockPos))
			if utils.IsBlockPassInteraction(blockInWay) {
				continue
			}

			// Iterate through each block's bounding boxes and check if it is in the way of the ray.
			for _, blockBB := range utils.BlockCollisions(blockInWay, flooredBlockPos, c.mPlayer.World()) {
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
		!c.useClientTracker && !hitValid && !c.checkMisprediction,
		"<red>hit was invalidated</red> (raycast=%f, raw=%f, angle=%f)",
		closestRaycastDist,
		closestRawDist,
		closestAngle,
	)

	// If this is the full-authoritative combat component, and the hit is valid, send the attack packet to the server.
	if hitValid {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, !c.useClientTracker, "<green>hit sent to the server</green> (raycast=%f raw=%f, angle=%f)", closestRaycastDist, closestRawDist, closestAngle)
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, c.checkMisprediction, "<yellow>client mispredicted air hit, but actually attacked entity</yellow>")
		if !c.useClientTracker {
			c.mPlayer.SendPacketToServer(c.attackInput)
		}
	}

	for _, hook := range c.hooks {
		hook(c)
	}
	return !c.checkMisprediction || hitValid
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

func (c *AuthoritativeCombatComponent) Angles() []float32 {
	return c.angleResults
}

// checkForMispredictedEntity returns true if an entity were found to be in the way of the combat component.
func (c *AuthoritativeCombatComponent) checkForMispredictedEntity() bool {
	if c.useClientTracker {
		return false
	}

	var (
		minDist        float32 = 1_000_000
		targetedEntity *entity.Entity
		rewindData     entity.HistoricalPosition
		eid            uint64
	)

	// We subtract the rewind tick by 1 here, because the client has already ticked in this instance (which increases)
	// the client tick by 1, so we have to rewind to the previous tick.
	rewTick := c.mPlayer.ClientTick - 1
	for rid, e := range c.entityTracker().All() {
		rewind, ok := e.Rewind(rewTick)
		if !ok {
			continue
		}

		dist := rewind.Position.Sub(c.endAttackPos).Len()
		if dist <= CombatSurvivalEntitySearchRadius {
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

	var newItem protocol.ItemInstance
	if c.mPlayer.LastEquipmentData != nil {
		newItem = c.mPlayer.LastEquipmentData.NewItem
	}

	c.attackInput = &packet.InventoryTransaction{
		TransactionData: &protocol.UseItemOnEntityTransactionData{
			TargetEntityRuntimeID: eid,
			ActionType:            protocol.UseItemOnEntityActionAttack,
			HotBarSlot:            c.mPlayer.Inventory().HeldSlot(),
			HeldItem:              newItem,
			Position:              c.endAttackPos,
			ClickedPosition:       mgl32.Vec3{},
		},
	}
	return true
}

func (c *AuthoritativeCombatComponent) entityTracker() player.EntityTrackerComponent {
	if c.useClientTracker {
		return c.mPlayer.ClientEntityTracker()
	}
	return c.mPlayer.EntityTracker()
}

func (c *AuthoritativeCombatComponent) Reset() {
	c.attackInput = nil
	c.targetedEntity = nil
	c.checkMisprediction = false
	c.raycastResults = c.raycastResults[:0]
	c.rawResults = c.rawResults[:0]
	c.angleResults = c.angleResults[:0]
	c.attacked = false
	for rid := range c.uniqueAttackedEntities {
		delete(c.uniqueAttackedEntities, rid)
	}
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
