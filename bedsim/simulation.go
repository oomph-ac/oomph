package bedsim

import (
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Simulate runs a movement simulation tick and returns the resulting state.
func (s *Simulator) Simulate(state *MovementState, input InputState) SimulationResult {
	if state == nil {
		return SimulationResult{}
	}

	s.applyInput(state, input)
	reason := s.simulateCore(state)
	if s.Options.SprintTiming == SprintTimingLegacy {
		s.applyLegacySprint(state, input)
	}
	s.tickState(state)
	return s.resultFromState(state, reason)
}

// SimulateState runs movement simulation using the current state values, without applying input updates
// or advancing tick counters. This is useful when the caller handles input parsing and ticking externally.
func (s *Simulator) SimulateState(state *MovementState) SimulationResult {
	if state == nil {
		return SimulationResult{}
	}
	reason := s.simulateCore(state)
	return s.resultFromState(state, reason)
}

func (s *Simulator) simulateCore(state *MovementState) SimulationOutcome {
	teleported := s.attemptTeleport(state)
	if teleported {
		return SimulationOutcomeTeleport
	}

	reliable := s.simulationIsReliable(state)
	if !reliable {
		s.resetToClient(state)
		return SimulationOutcomeUnreliable
	}
	if s.World != nil && !s.World.IsChunkLoaded(int32(state.Pos.X())>>4, int32(state.Pos.Z())>>4) {
		state.SetVel(mgl64.Vec3{})
		return SimulationOutcomeUnloadedChunk
	}
	if state.Immobile || !state.Ready {
		state.SetVel(mgl64.Vec3{})
		return SimulationOutcomeImmobileOrNotReady
	}

	s.simulateMovement(state)
	return SimulationOutcomeNormal
}

func (s *Simulator) resultFromState(state *MovementState, outcome SimulationOutcome) SimulationResult {
	result := SimulationResult{
		Position: state.Pos,
		Velocity: state.Vel,
		Movement: state.Mov,
		OnGround: state.OnGround,
		CollideX: state.CollideX,
		CollideY: state.CollideY,
		CollideZ: state.CollideZ,
		Outcome:  outcome,
	}

	result.PositionDelta = state.Pos.Sub(state.Client.Pos)
	result.VelocityDelta = state.Vel.Sub(state.Client.Vel)

	needsPos := s.Options.PositionCorrectionThreshold > 0 && result.PositionDelta.Len() > s.Options.PositionCorrectionThreshold
	needsVel := s.Options.VelocityCorrectionThreshold > 0 && result.VelocityDelta.Len() > s.Options.VelocityCorrectionThreshold
	result.NeedsCorrection = needsPos || needsVel

	return result
}

func (s *Simulator) applyInput(state *MovementState, input InputState) {
	state.Client.HorizontalCollision = input.HorizontalCollision
	state.Client.VerticalCollision = input.VerticalCollision

	state.Client.LastPos = state.Client.Pos
	state.Client.Pos = input.ClientPos
	state.Client.LastVel = state.Client.Vel
	state.Client.Vel = input.ClientVel
	state.Client.LastMov = state.Client.Mov
	state.Client.Mov = state.Client.Pos.Sub(state.Client.LastPos)

	if input.StartFlying {
		state.Client.ToggledFly = true
		if state.TrustFlyStatus {
			state.Flying = true
		}
	} else if input.StopFlying {
		if state.Flying {
			state.JustDisabledFlight = true
		}
		state.Flying = false
		state.Client.ToggledFly = false
	}

	state.SetRotation(mgl64.Vec3{input.Pitch, input.HeadYaw, input.Yaw})

	state.PressingSneak = input.Sneaking
	state.PressingSprint = input.SprintDown

	startFlag, stopFlag := input.StartSprinting, input.StopSprinting
	needsSpeedAdjusted := false
	isModernSprint := s.Options.SprintTiming == SprintTimingModern
	if startFlag && stopFlag {
		needsSpeedAdjusted = isModernSprint
		state.Sprinting = false
		state.AirSpeed = 0.02
	} else if !startFlag && !stopFlag && !state.ServerSprintApplied && state.ServerSprint != state.Sprinting {
		if state.ServerSprint {
			state.Sprinting = true
			state.AirSpeed = 0.026
		} else {
			state.Sprinting = false
			state.AirSpeed = 0.02
		}
	} else if startFlag {
		state.Sprinting = true
		needsSpeedAdjusted = isModernSprint
		state.AirSpeed = 0.026
	} else if stopFlag {
		state.Sprinting = false
		needsSpeedAdjusted = isModernSprint && !state.ServerUpdatedSpeed
		state.AirSpeed = 0.02
	}
	state.ServerSprintApplied = true

	if needsSpeedAdjusted {
		state.ServerUpdatedSpeed = false
		state.MovementSpeed = state.DefaultMovementSpeed
		if state.Sprinting {
			state.MovementSpeed *= 1.3
		}
	}

	if input.StartSneaking {
		state.Sneaking = true
	} else if input.StopSneaking {
		state.Sneaking = false
	} else {
		state.Sneaking = input.SneakDown
	}

	maxImpulse := 1.0
	if input.UsingConsumable {
		maxImpulse *= MaxConsumingImpulse
	}
	if state.Sneaking {
		maxImpulse *= MaxSneakImpulse
	}

	moveVector := mgl64.Vec2{
		ClampFloat(input.MoveVector[0], -maxImpulse, maxImpulse),
		ClampFloat(input.MoveVector[1], -maxImpulse, maxImpulse),
	}

	state.Jumping = input.StartJumping
	state.PressingJump = input.Jumping
	state.JumpHeight = DefaultJumpHeight
	if s.Effects != nil {
		if amp, ok := s.Effects.GetEffect(packet.EffectJumpBoost); ok {
			state.JumpHeight += float64(amp) * 0.1
		}
	}

	if !state.PressingJump {
		state.JumpDelay = 0
	}
	state.Gravity = NormalGravity

	if input.StopGliding {
		state.Gliding = false
		state.GlideBoostTicks = 0
	} else if input.StartGliding {
		state.Gliding = true
	}

	state.Impulse = moveVector.Mul(0.98)
}

func (s *Simulator) applyLegacySprint(state *MovementState, input InputState) {
	needsSpeedAdjusted := false
	if input.StartSprinting && input.StopSprinting {
		state.Sprinting = false
		needsSpeedAdjusted = true
	} else if input.StartSprinting {
		state.Sprinting = true
		needsSpeedAdjusted = true
	} else if input.StopSprinting {
		state.Sprinting = false
		needsSpeedAdjusted = !state.ServerUpdatedSpeed
	}
	if needsSpeedAdjusted {
		state.ServerUpdatedSpeed = false
		state.MovementSpeed = state.DefaultMovementSpeed
		if state.Sprinting {
			state.MovementSpeed *= 1.3
		}
	}
}

func (s *Simulator) tickState(state *MovementState) {
	state.GlideBoostTicks--
	state.TicksSinceKnockback++
	if state.TicksSinceTeleport < math.MaxUint64 {
		state.TicksSinceTeleport++
	}
	if state.JumpDelay > 0 {
		state.JumpDelay--
	}
	state.JustDisabledFlight = false
}

func (s *Simulator) simulateMovement(state *MovementState) {
	if state.Vel.LenSqr() < 1e-12 {
		state.SetVel(mgl64.Vec3{})
	}

	blockUnder := s.blockAtPos(cube.PosFromVec3(state.Pos.Sub(mgl64.Vec3{0, 0.5})))
	blockFriction := DefaultAirFriction
	moveRelativeSpeed := state.AirSpeed
	if state.OnGround {
		mSpeed := state.MovementSpeed
		if BlockName(blockUnder) == "minecraft:soul_sand" {
			mSpeed *= 0.543
		}
		blockFriction *= BlockFriction(blockUnder)
		moveRelativeSpeed = mSpeed * (0.16277136 / (blockFriction * blockFriction * blockFriction))
	}

	if state.Gliding {
		if s.Inventory != nil && s.Inventory.HasElytra() && !state.OnGround {
			state.OnGround = false
			simulateGlide(state)
			tryCollisions(state, s.World, s.Options.UseSlideOffset, s.Options.PositionCorrectionThreshold, false)
			state.SetMov(state.Vel)
		} else {
			state.Gliding = false
		}
		return
	}

	var clientJumpPrevented bool
	attemptKnockback(state)
	moveRelative(state, moveRelativeSpeed)
	attemptJump(state, &clientJumpPrevented, s)

	nearClimbable := BlockClimbable(s.blockAtPos(cube.PosFromVec3(state.Pos)))
	if nearClimbable {
		newVel := state.Vel
		negClimbSpeed := -ClimbSpeed
		if newVel[1] < negClimbSpeed {
			newVel[1] = negClimbSpeed
		}
		if state.PressingJump || state.CollideX || state.CollideZ {
			newVel[1] = ClimbSpeed
		}
		if state.Sneaking && newVel[1] < 0 {
			newVel[1] = 0
		}
		state.SetVel(newVel)
	}

	blocksInside, isInsideBlock := s.blocksInside(state)
	inCobweb := false
	if isInsideBlock {
		for _, b := range blocksInside {
			if BlockName(b) == "minecraft:web" {
				inCobweb = true
				break
			}
		}
	}

	if inCobweb {
		newVel := state.Vel
		newVel[0] *= 0.25
		newVel[1] *= 0.05
		newVel[2] *= 0.25
		state.SetVel(newVel)
	}

	avoidEdge(state, s.World, s.Options.UseSlideOffset)

	oldVel := state.Vel
	oldOnGround := state.OnGround
	oldY := state.Pos.Y()

	tryCollisions(state, s.World, s.Options.UseSlideOffset, s.Options.PositionCorrectionThreshold, clientJumpPrevented)
	if state.SupportingBlockPos != nil {
		blockUnder = s.blockAtPos(*state.SupportingBlockPos)
	} else {
		blockUnder = s.blockAtPos(cube.PosFromVec3(state.Pos.Sub(mgl64.Vec3{0, 0.2})))
		if _, isAir := blockUnder.(block.Air); isAir {
			below := s.blockAtPos(cube.PosFromVec3(state.Pos).Side(cube.FaceDown))
			if IsWall(below) || IsFence(below) {
				blockUnder = below
			}
		}
	}

	if oldY == state.Pos.Y() {
		walkOnBlock(state, blockUnder)
	}

	state.SetMov(state.Vel)
	setPostCollisionMotion(state, oldVel, oldOnGround, blockUnder)

	if inCobweb {
		state.SetVel(mgl64.Vec3{})
	}

	newVel := state.Vel
	if s.Effects != nil {
		if amp, ok := s.Effects.GetEffect(packet.EffectLevitation); ok {
			levSpeed := LevitationGravityMultiplier * float64(amp)
			newVel[1] += (levSpeed - newVel[1]) * 0.2
		} else if state.HasGravity {
			newVel[1] -= state.Gravity
			newVel[1] *= NormalGravityMultiplier
		}
	} else if state.HasGravity {
		newVel[1] -= state.Gravity
		newVel[1] *= NormalGravityMultiplier
	}
	newVel[0] *= blockFriction
	newVel[2] *= blockFriction
	state.SetVel(newVel)
}

func (s *Simulator) simulationIsReliable(state *MovementState) bool {
	if state.RemainingTeleportTicks() > 0 {
		return true
	}

	bb := state.BoundingBox(s.Options.UseSlideOffset).Grow(1)
	for _, result := range nearbyBlocks(bb, s.World) {
		if _, isAir := result.block.(block.Air); isAir {
			continue
		}
		if _, isLiquid := result.block.(world.Liquid); isLiquid {
			blockBB := cube.Box(0, 0, 0, 1, 1, 1).Translate(result.pos.Vec3())
			if state.BoundingBox(s.Options.UseSlideOffset).IntersectsWith(blockBB) {
				return false
			}
		}
		if BlockName(result.block) == "minecraft:bamboo" {
			return false
		}
	}

	if state.GameMode != packet.GameTypeSurvival && state.GameMode != packet.GameTypeAdventure {
		return false
	}
	if state.Flying || state.JustDisabledFlight || state.NoClip || !state.Alive {
		return false
	}
	return true
}

func (s *Simulator) resetToClient(state *MovementState) {
	state.LastPos = state.Client.LastPos
	state.Pos = state.Client.Pos
	state.LastVel = state.Client.LastVel
	state.Vel = state.Client.Vel
	state.LastMov = state.Client.LastMov
	state.Mov = state.Client.Mov
	if state.Flying {
		state.OnGround = false
	}

	if s.Options.LimitAllVelocity {
		limit := s.Options.LimitAllVelocityThreshold
		if limit < 0 {
			limit = -limit
		}
		state.Vel[0] = ClampFloat(state.Vel[0], -limit, limit)
		state.Vel[1] = ClampFloat(state.Vel[1], -limit, limit)
		state.Vel[2] = ClampFloat(state.Vel[2], -limit, limit)
	}
}

func (s *Simulator) attemptTeleport(state *MovementState) bool {
	if !state.HasTeleport() {
		return false
	}

	if !state.TeleportIsSmoothed {
		state.SetPos(state.TeleportPos)
		state.SetVel(mgl64.Vec3{})
		state.JumpDelay = 0
		attemptJump(state, nil, s)
		return true
	}

	posDelta := state.TeleportPos.Sub(state.Pos)
	if remaining := state.RemainingTeleportTicks() + 1; remaining > 0 {
		newPos := state.Pos.Add(posDelta.Mul(1.0 / float64(remaining)))
		state.SetPos(newPos)
		state.JumpDelay = 0
		return remaining > 1
	}
	return false
}

func simulateGlide(state *MovementState) {
	radians := math.Pi / 180.0
	yaw, pitch := state.Rotation.Z()*radians, state.Rotation.X()*radians
	yawCos := math.Cos(-yaw - math.Pi)
	yawSin := math.Sin(-yaw - math.Pi)
	pitchCos := math.Cos(pitch)
	pitchSin := math.Sin(pitch)

	lookX := yawSin * -pitchCos
	lookY := -pitchSin
	lookZ := yawCos * -pitchCos

	vel := state.Vel
	velHz := math.Sqrt(vel[0]*vel[0] + vel[2]*vel[2])
	lookHz := pitchCos
	sqrPitchCos := pitchCos * pitchCos

	vel[1] += -0.08 + sqrPitchCos*0.06
	if vel[1] < 0 && lookHz > 0 {
		yAccel := vel[1] * -0.1 * sqrPitchCos
		vel[1] += yAccel
		vel[0] += lookX * yAccel / lookHz
		vel[2] += lookZ * yAccel / lookHz
	}
	if pitch < 0 {
		yAccel := velHz * -pitchSin * 0.04
		vel[1] += yAccel * 3.2
		vel[0] -= lookX * yAccel / lookHz
		vel[2] -= lookZ * yAccel / lookHz
	}
	if lookHz > 0 {
		vel[0] += (lookX/lookHz*velHz - vel[0]) * 0.1
		vel[2] += (lookZ/lookHz*velHz - vel[2]) * 0.1
	}

	if state.GlideBoostTicks > 0 {
		vel[0] += (lookX * 0.1) + (((lookX * 1.5) - vel[0]) * 0.5)
		vel[1] += (lookY * 0.1) + (((lookY * 1.5) - vel[1]) * 0.5)
		vel[2] += (lookZ * 0.1) + (((lookZ * 1.5) - vel[2]) * 0.5)
	}

	vel[0] *= 0.99
	vel[1] *= 0.98
	vel[2] *= 0.99

	state.SetVel(vel)
}

func walkOnBlock(state *MovementState, blockUnder world.Block) {
	if !state.OnGround || state.Sneaking {
		return
	}

	newVel := state.Vel
	switch BlockName(blockUnder) {
	case "minecraft:slime":
		yMov := math.Abs(newVel.Y())
		if yMov < 0.1 && !state.PressingSneak {
			d1 := 0.4 + yMov*0.2
			newVel[0] *= d1
			newVel[2] *= d1
		}
	}
	state.SetVel(newVel)
}

func landOnBlock(state *MovementState, old mgl64.Vec3, blockUnder world.Block) {
	newVel := state.Vel
	if old.Y() >= 0 || state.PressingSneak {
		newVel[1] = 0
		state.SetVel(newVel)
		return
	}

	switch BlockName(blockUnder) {
	case "minecraft:slime":
		newVel[1] = SlimeBounceMultiplier * old.Y()
		if math.Abs(newVel[1]) < 1e-4 {
			newVel[1] = 0.0
		}
	case "minecraft:bed":
		newVel[1] = math.Min(1.0, BedBounceMultiplier*old.Y())
	default:
		newVel[1] = 0
	}
	state.SetVel(newVel)
}

func setPostCollisionMotion(state *MovementState, oldVel mgl64.Vec3, oldOnGround bool, blockUnder world.Block) {
	if !oldOnGround && state.CollideY {
		landOnBlock(state, oldVel, blockUnder)
	} else if state.CollideY {
		newVel := state.Vel
		newVel[1] = 0
		state.SetVel(newVel)
	}

	newVel := state.Vel
	if state.CollideX {
		newVel[0] = 0
	}
	if state.CollideZ {
		newVel[2] = 0
	}
	state.SetVel(newVel)
}

func moveRelative(state *MovementState, moveRelativeSpeed float64) {
	impulse := state.Impulse
	force := impulse.Y()*impulse.Y() + impulse.X()*impulse.X()

	if force >= 1e-4 {
		force = moveRelativeSpeed / math.Max(math.Sqrt(force), 1.0)
		mf, ms := impulse.Y()*force, impulse.X()*force

		yaw := state.Rotation.Z() * math.Pi / 180.0
		v2, v3 := MCSin(yaw), MCCos(yaw)

		newVel := state.Vel
		newVel[0] += ms*v3 - mf*v2
		newVel[2] += mf*v3 + ms*v2
		state.SetVel(newVel)
	}
}

func attemptKnockback(state *MovementState) bool {
	if state.HasKnockback() {
		state.SetVel(state.Knockback)
		return true
	}
	return false
}

func attemptJump(state *MovementState, clientJumpPrevented *bool, sim *Simulator) bool {
	if !state.Jumping || !state.OnGround || state.JumpDelay > 0 {
		return false
	}

	newVel := state.Vel
	newVel[1] = math.Max(state.JumpHeight, newVel[1])
	state.JumpDelay = JumpDelayTicks

	if state.Sprinting {
		force := state.Rotation.Z() * 0.017453292
		newVel[0] -= MCSin(force) * 0.2
		newVel[2] += MCCos(force) * 0.2
	}

	if clientJumpPrevented != nil && !state.HasKnockback() && !state.HasTeleport() {
		if sim != nil && isJumpBlocked(state, sim.World, sim.Options.UseSlideOffset, newVel) {
			*clientJumpPrevented = true
		}
	}

	state.SetVel(newVel)
	return true
}

func isJumpBlocked(state *MovementState, w WorldProvider, useSlideOffset bool, jumpVel mgl64.Vec3) bool {
	if w == nil {
		return false
	}
	collisionBB := state.BoundingBox(useSlideOffset)
	bbList := w.GetNearbyBBoxes(collisionBB.Extend(jumpVel))

	yVel := mgl64.Vec3{0, jumpVel.Y()}
	xVel := mgl64.Vec3{jumpVel.X()}
	zVel := mgl64.Vec3{0, 0, jumpVel.Z()}

	for i := len(bbList) - 1; i >= 0; i-- {
		yVel = BBClipCollide(bbList[i], collisionBB, yVel, false, nil)
	}
	collisionBB = collisionBB.Translate(yVel)

	for i := len(bbList) - 1; i >= 0; i-- {
		xVel = BBClipCollide(bbList[i], collisionBB, xVel, false, nil)
	}
	collisionBB = collisionBB.Translate(xVel)

	for i := len(bbList) - 1; i >= 0; i-- {
		zVel = BBClipCollide(bbList[i], collisionBB, zVel, false, nil)
	}
	initialBlockCond := ((xVel[0] != jumpVel[0]) || (zVel[2] != jumpVel[2])) && yVel[1] == jumpVel[1]
	if !initialBlockCond {
		return false
	}

	xVel = mgl64.Vec3{jumpVel.X()}
	yVel = mgl64.Vec3{0, jumpVel.Y()}
	zVel = mgl64.Vec3{0, 0, jumpVel.Z()}
	collisionBB = state.BoundingBox(useSlideOffset)

	for i := len(bbList) - 1; i >= 0; i-- {
		xVel = BBClipCollide(bbList[i], collisionBB, xVel, false, nil)
	}
	collisionBB = collisionBB.Translate(xVel)

	for i := len(bbList) - 1; i >= 0; i-- {
		zVel = BBClipCollide(bbList[i], collisionBB, zVel, false, nil)
	}
	collisionBB = collisionBB.Translate(zVel)

	for i := len(bbList) - 1; i >= 0; i-- {
		yVel = BBClipCollide(bbList[i], collisionBB, yVel, false, nil)
	}
	return yVel[1] != jumpVel[1] && xVel[0] == jumpVel[0] && zVel[2] == jumpVel[2]
}

func tryCollisions(state *MovementState, w WorldProvider, useSlideOffset bool, correctionThreshold float64, clientJumpPrevented bool) {
	if w == nil {
		return
	}

	var completedStep bool
	collisionBB := state.BoundingBox(useSlideOffset)
	currVel := state.Vel
	bbList := w.GetNearbyBBoxes(collisionBB.Extend(currVel))

	useOneWayCollisions := state.StuckInCollider
	penetration := mgl64.Vec3{}

	yVel := mgl64.Vec3{0, currVel.Y()}
	if clientJumpPrevented {
		yVel[1] = 0
	}
	xVel := mgl64.Vec3{currVel.X()}
	zVel := mgl64.Vec3{0, 0, currVel.Z()}

	for i := len(bbList) - 1; i >= 0; i-- {
		yVel = BBClipCollide(bbList[i], collisionBB, yVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(yVel)

	for i := len(bbList) - 1; i >= 0; i-- {
		xVel = BBClipCollide(bbList[i], collisionBB, xVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(xVel)

	for i := len(bbList) - 1; i >= 0; i-- {
		zVel = BBClipCollide(bbList[i], collisionBB, zVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(zVel)

	collisionVel := yVel.Add(xVel).Add(zVel)
	collisionPos := mgl64.Vec3{
		(collisionBB.Min().X() + collisionBB.Max().X()) * 0.5,
		collisionBB.Min().Y(),
		(collisionBB.Min().Z() + collisionBB.Max().Z()) * 0.5,
	}

	hasPenetration := penetration.LenSqr() >= 9.999999999999999e-12
	state.StuckInCollider = state.PenetratedLastFrame && hasPenetration
	state.PenetratedLastFrame = hasPenetration

	xCollision := currVel.X() != collisionVel.X()
	yCollision := (currVel.Y() != collisionVel.Y()) || clientJumpPrevented
	zCollision := currVel.Z() != collisionVel.Z()
	onGround := state.OnGround || (yCollision && currVel.Y() < 0.0)

	if onGround && (xCollision || zCollision) {
		stepYVel := mgl64.Vec3{0, StepHeight}
		stepXVel := mgl64.Vec3{currVel.X()}
		stepZVel := mgl64.Vec3{0, 0, currVel.Z()}

		stepBB := state.BoundingBox(useSlideOffset)
		for _, blockBox := range bbList {
			stepYVel = BBClipCollide(blockBox, stepBB, stepYVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(stepYVel)

		for _, blockBox := range bbList {
			stepXVel = BBClipCollide(blockBox, stepBB, stepXVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(stepXVel)

		for _, blockBox := range bbList {
			stepZVel = BBClipCollide(blockBox, stepBB, stepZVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(stepZVel)

		inverseYStepVel := stepYVel.Mul(-1)
		for _, blockBox := range bbList {
			inverseYStepVel = BBClipCollide(blockBox, stepBB, inverseYStepVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(inverseYStepVel)
		stepYVel = stepYVel.Add(inverseYStepVel)

		stepVel := stepYVel.Add(stepXVel).Add(stepZVel)
		newBBList := w.GetNearbyBBoxes(stepBB)
		stepPos := mgl64.Vec3{
			(stepBB.Min().X() + stepBB.Max().X()) * 0.5,
			stepBB.Min().Y(),
			(stepBB.Min().Z() + stepBB.Max().Z()) * 0.5,
		}

		if len(newBBList) == 0 && Vec3HzDistSqr(collisionVel) < Vec3HzDistSqr(stepVel) {
			stepPosDist := stepPos.Sub(state.Client.Pos).Len()
			collisionPosDist := collisionPos.Sub(state.Client.Pos).Len()
			if collisionPosDist > correctionThreshold || stepPosDist <= collisionPosDist {
				collisionVel = stepVel
				collisionBB = stepBB
				if useSlideOffset {
					completedStep = true
					slideOffset := state.SlideOffset.Mul(SlideOffsetMultiplier)
					slideOffset[1] += stepVel.Y()
					state.SlideOffset = slideOffset
				}
			}
		}
	}

	endPos := mgl64.Vec3{
		(collisionBB.Min().X() + collisionBB.Max().X()) * 0.5,
		collisionBB.Min().Y(),
		(collisionBB.Min().Z() + collisionBB.Max().Z()) * 0.5,
	}

	if useSlideOffset {
		if completedStep {
			endPos[1] -= state.SlideOffset.Y()
		} else {
			state.SlideOffset = mgl64.Vec2{}
		}
	}
	state.SetPos(endPos)

	yCollision = math.Abs(currVel.Y()-collisionVel.Y()) >= 1e-5
	state.CollideX = math.Abs(currVel.X()-collisionVel.X()) >= 1e-5
	state.CollideY = yCollision
	state.CollideZ = math.Abs(currVel.Z()-collisionVel.Z()) >= 1e-5

	state.OnGround = (yCollision && currVel.Y() < 0) || (state.OnGround && !yCollision && math.Abs(currVel.Y()) <= 1e-5)
	checkSupportingBlockPos(state, w, useSlideOffset, currVel)
	state.SetVel(collisionVel)
}

func avoidEdge(state *MovementState, w WorldProvider, useSlideOffset bool) {
	if w == nil {
		return
	}
	if !state.Sneaking || !state.OnGround || state.Vel.Y() > 0 {
		return
	}

	edgeBoundry := 0.025
	offset := 0.05

	newVel := state.Vel
	bb := state.BoundingBox(useSlideOffset).GrowVec3(mgl64.Vec3{-edgeBoundry, 0, -edgeBoundry})
	xMov, zMov := newVel.X(), newVel.Z()

	for xMov != 0.0 && len(w.GetNearbyBBoxes(bb.Translate(mgl64.Vec3{xMov, -StepHeight * 1.01, 0}))) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}
	}

	for zMov != 0.0 && len(w.GetNearbyBBoxes(bb.Translate(mgl64.Vec3{0, -StepHeight * 1.01, zMov}))) == 0 {
		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	for xMov != 0.0 && zMov != 0.0 && len(w.GetNearbyBBoxes(bb.Translate(mgl64.Vec3{xMov, -StepHeight * 1.01, zMov}))) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}

		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	newVel[0] = xMov
	newVel[2] = zMov
	state.SetVel(newVel)
}

func (s *Simulator) blocksInside(state *MovementState) ([]world.Block, bool) {
	bb := state.BoundingBox(s.Options.UseSlideOffset)
	blocks := []world.Block{}

	for _, result := range nearbyBlocks(bb.Grow(1), s.World) {
		pos := result.pos
		b := result.block
		if _, isAir := b.(block.Air); isAir {
			continue
		}

		boxes := s.World.BlockCollisions(pos)
		for _, box := range boxes {
			if bb.IntersectsWith(box.Translate(pos.Vec3())) {
				blocks = append(blocks, b)
				break
			}
		}
	}

	return blocks, len(blocks) > 0
}

type blockResult struct {
	pos   cube.Pos
	block world.Block
}

func nearbyBlocks(aabb cube.BBox, w WorldProvider) []blockResult {
	if w == nil {
		return nil
	}
	min, max := aabb.Min(), aabb.Max()
	minX, minY, minZ := int(math.Floor(min[0])), int(math.Floor(min[1])), int(math.Floor(min[2]))
	maxX, maxY, maxZ := int(math.Ceil(max[0])), int(math.Ceil(max[1])), int(math.Ceil(max[2]))

	blocks := make([]blockResult, 0, (maxX-minX)*(maxY-minY)*(maxZ-minZ))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				b := w.Block(pos)
				blocks = append(blocks, blockResult{pos: pos, block: b})
			}
		}
	}
	return blocks
}

func checkSupportingBlockPos(state *MovementState, w WorldProvider, useSlideOffset bool, vel mgl64.Vec3) {
	if !state.OnGround {
		state.SupportingBlockPos = nil
		return
	}
	decBB := state.BoundingBox(useSlideOffset).ExtendTowards(cube.FaceDown, 1e-3)
	findSupportingBlock(state, w, decBB)
	if state.SupportingBlockPos == nil {
		decBB = decBB.Translate(mgl64.Vec3{-vel[0], 0, -vel[2]})
		findSupportingBlock(state, w, decBB)
	}
}

func findSupportingBlock(state *MovementState, w WorldProvider, bb cube.BBox) {
	if w == nil {
		return
	}
	var blockPos *cube.Pos
	minDist := math.MaxFloat64 - 1
	centerPos := cube.PosFromVec3(state.Pos).Vec3().Add(mgl64.Vec3{0.5, 0.5, 0.5})

	for _, result := range nearbyBlocks(bb, w) {
		boxes := w.BlockCollisions(result.pos)
		if len(boxes) == 0 {
			continue
		}

		for _, box := range boxes {
			if !bb.IntersectsWith(box.Translate(result.pos.Vec3())) {
				continue
			}
			dist := result.pos.Vec3().Sub(centerPos).LenSqr()
			if dist < minDist {
				minDist = dist
				pos := result.pos
				blockPos = &pos
			}
			break
		}
	}

	state.SupportingBlockPos = blockPos
}

func (s *Simulator) blockAtPos(pos cube.Pos) world.Block {
	if s.World == nil {
		return block.Air{}
	}
	return s.World.Block(pos)
}
