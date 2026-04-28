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

// Defaults used as fallback when the matching LocalCombatOpts field is unset
// (see per-field docs for the sentinel each one checks).
const (
	CombatLerpPositionSteps                  = 10
	CombatSurvivalEntitySearchRadius float32 = 6.0
	CombatSurvivalReach              float32 = 2.9
	CombatDefaultBBoxExpansion       float32 = 0.1
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

	// lerpSteps is captured at construction so result-slice caps stay correct
	// and Calculate doesn't re-read it each tick. Runtime LerpSteps mutation
	// therefore takes effect next session (documented on LocalCombatOpts).
	lerpSteps int
}

func NewAuthoritativeCombatComponent(p *player.Player, useClientTracker bool) *AuthoritativeCombatComponent {
	steps := CombatLerpPositionSteps
	if cfg := p.Opts(); cfg != nil && cfg.LocalCombat.LerpSteps > 0 {
		steps = cfg.LocalCombat.LerpSteps
	}
	sliceCap := (steps + 1) * 2

	return &AuthoritativeCombatComponent{
		mPlayer:                p,
		raycastResults:         make([]float32, 0, sliceCap),
		rawResults:             make([]float32, 0, sliceCap),
		angleResults:           make([]float32, 0, sliceCap),
		hooks:                  []player.CombatHook{},
		uniqueAttackedEntities: make(map[uint64]*entity.Entity),

		useClientTracker: useClientTracker,
		lerpSteps:        steps,
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
		c.mPlayer.ResetTicksSinceAttack()

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

	if c.mPlayer.Dbg.Enabled(player.DebugModeCombat) {
		t := time.Now()
		defer func() {
			delta := float64(time.Since(t).Nanoseconds()) / 1_000_000.0
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "took %.4fms to process", delta)
		}()
	}

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
	if c.useClientTracker {
		if pending := movement.PendingCorrections(); pending > 0 {
			c.mPlayer.Dbg.Notify(player.DebugModeCombat, true, "movement component indicates pending corrections (%d) - hit invalidated", pending)
			return false
		}
	}

	if movement.HasTeleport() && !movement.TeleportSmoothed() {
		teleportPos := movement.TeleportPos()
		c.startAttackPos = teleportPos
		c.endAttackPos = teleportPos
	}

	c.startRotation = movement.LastRotation()
	c.endRotation = movement.Rotation()

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

	local := c.mPlayer.Opts().LocalCombat
	lerpSteps := c.lerpSteps // captured at construction; see field doc
	// 0 is honoured as "exact bbox" for BBoxExpansion; only negative values
	// fall back to the default.
	bboxGrow := local.BBoxExpansion
	if bboxGrow < 0 {
		bboxGrow = CombatDefaultBBoxExpansion
	}
	maxReach := local.MaximumReach
	if maxReach <= 0 {
		maxReach = CombatSurvivalReach
	}
	raycastReach := maxReach + local.ReachLeniency

	stepAmt := 1.0 / float32(lerpSteps)

	for step := 0; step <= lerpSteps; step++ {
		partialTicks := float32(step) * stepAmt

		lerpedResult := c.lerp(partialTicks)
		entityBB := c.entityBB.Translate(lerpedResult.entityPos).Grow(bboxGrow)
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
			hitValid = hitValid || raycastDist <= raycastReach
			c.raycastResults = append(c.raycastResults, raycastDist)

			if raycastDist < closestRaycastDist {
				closestRaycastDist = raycastDist
				closestHitResult = hitResult
				lerpedAtClosest = lerpedResult
			}
			raycastHit = true
		}

		// An extra raycast is ran here with the current entity position, as the client may have ticked
		// the entity to a new position while the frame logic was running (where attacks are done). This is
		// only required when running on the client tracker, as we want to match the client's own logic as
		// closely as possible.
		if c.useClientTracker {
			altEntPos := altEntityStartPos.Add(altEntityPosDelta.Mul(partialTicks))
			altEntityBB := c.entityBB.Translate(altEntPos).Grow(bboxGrow)

			altAngle := math32.Abs(game.AngleToPoint(lerpedResult.attackPos, altEntPos, lerpedResult.rotation)[0])
			c.angleResults = append(c.angleResults, altAngle)
			if altAngle < closestAngle {
				closestAngle = altAngle
			}

			if hitResult, ok := trace.BBoxIntercept(altEntityBB, lerpedResult.attackPos, lerpedResult.attackPos.Add(dV.Mul(7.0))); ok {
				altRaycastDist := lerpedResult.attackPos.Sub(hitResult.Position()).Len()
				hitValid = hitValid || altRaycastDist <= raycastReach
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

	// Touch players (or non-touch with RawDistanceFallback opted in) accept
	// the closest-point distance when no raycast lands. Gated by reach/angle
	// to keep behind-the-player and input-spoof advantages off the table.
	if !hitValid && (c.mPlayer.InputMode == packet.InputModeTouch || local.RawDistanceFallback) {
		hitValid = closestRawDist <= maxReach && closestAngle <= c.mPlayer.Opts().Combat.MaximumAttackAngle
		if hitValid {
			lerpedAtClosest.attackPos = c.endAttackPos
			lerpedAtClosest.entityPos = c.startEntityPos
			lerpedAtClosest.rotation = c.endRotation
			utils.ModifyBBoxResult(&closestHitResult, c.entityBB, closestRawPos, 0)
			raycastHit = true
		}
	}

	// Reject hits whose ray passes through a solid block. Common false-positive
	// source on client/server block-state desync, hence the opt-out.
	if !c.useClientTracker && hitValid && raycastHit && closestRaycastDist > 0 && !local.DisableBlockOcclusionCheck {
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

	// DisableFullAuthoritative forwards rejected hits to the server (hooks still
	// flag). Mispredictions are deliberately excluded — forwarding their
	// synthesised packet would let a killaura-as-air-swing register on every
	// nearby target. Misprediction stays gated on hitValid as the anti-cheat floor.
	forwardHit := hitValid
	if !c.useClientTracker && local.DisableFullAuthoritative && !c.checkMisprediction && !hitValid {
		forwardHit = true
		c.mPlayer.Dbg.Notify(
			player.DebugModeCombat,
			true,
			"<yellow>full-auth off: forwarding rejected hit</yellow> (raycast=%f, raw=%f, angle=%f)",
			closestRaycastDist,
			closestRawDist,
			closestAngle,
		)
	}

	// If this is the full-authoritative combat component, and the hit is valid (or gating is off), send the attack packet to the server.
	if forwardHit {
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, !c.useClientTracker && hitValid, "<green>hit sent to the server</green> (raycast=%f raw=%f, angle=%f)", closestRaycastDist, closestRawDist, closestAngle)
		c.mPlayer.Dbg.Notify(player.DebugModeCombat, c.checkMisprediction && hitValid, "<yellow>client mispredicted air hit, but actually attacked entity</yellow>")
		if !c.useClientTracker && c.attackInput != nil {
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
		targetedEntity *entity.Entity
		rewindData     entity.HistoricalPosition
		eid            uint64
	)

	// 0 is honoured as "don't search"; only negative falls back to the default.
	radius := c.mPlayer.Opts().LocalCombat.EntitySearchRadius
	if radius < 0 {
		radius = CombatSurvivalEntitySearchRadius
	}
	radiusSq := radius * radius
	var minDistSq float32 = 1_000_000

	// We subtract the rewind tick by 1 here, because the client has already ticked in this instance (which increases)
	// the client tick by 1, so we have to rewind to the previous tick.
	rewTick := c.mPlayer.ClientTick - 1
	attackPos := c.endAttackPos
	for rid, e := range c.entityTracker().All() {
		rewind, ok := e.Rewind(rewTick)
		if !ok {
			continue
		}

		dx := rewind.Position[0] - attackPos[0]
		dy := rewind.Position[1] - attackPos[1]
		dz := rewind.Position[2] - attackPos[2]
		distSq := dx*dx + dy*dy + dz*dz
		if distSq <= radiusSq && distSq < minDistSq {
			minDistSq = distSq
			rewindData = rewind
			targetedEntity = e
			eid = rid
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
