package player

import (
	"fmt"
	"strings"

	"github.com/ethaniccc/float32-cube/cube"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// doMovementSimulation starts running the movement simulation of the player - this function will check if any exemptions need to be made.
// If no exemptions are needed, then this function will proceed to calculate the expected movement and position of the player this simulation frame.
// If a difference between the client calculated movement and server calculated are found, a correction will be sent.
func (p *Player) doMovementSimulation() {
	var exempt bool

	p.TryDebug(fmt.Sprintf("%v started movement simulation for frame %d", p.Name(), p.ClientFrame()), DebugTypeLogged, p.debugger.LogMovement)
	defer p.TryDebug(fmt.Sprintf("%v finished movement simulation for frame %d", p.Name(), p.ClientFrame()), DebugTypeLogged, p.debugger.LogMovement)

	surrounding := utils.GetNearbyBlocks(p.AABB(), true, false, p.World())
	if p.mInfo.LastBlocksSurrounding == nil {
		p.mInfo.LastBlocksSurrounding = surrounding
	}

	// If the player is AFK, do not run a movement simulation.
	/* afk := math32.Abs(p.mInfo.ForwardImpulse) == 0 && math32.Abs(p.mInfo.LeftImpulse) == 0 &&
	p.mInfo.ServerMovement == mgl32.Vec3{0, -0.0784, 0} && !p.mInfo.JumpBindPressed && !p.mInfo.SneakBindPressed &&
	!p.mInfo.SprintBindPressed && p.mInfo.TicksSinceKnockback > 0 && p.mInfo.TicksSinceSmoothTeleport > 3 &&
	!p.mInfo.Teleporting && maps.Equal(p.mInfo.LastBlocksSurrounding, surrounding) */
	afk := false

	if (p.movementMode == utils.ModeSemiAuthoritative && (p.inLoadedChunkTicks <= 5 || !p.ready)) || p.inDimensionChange || p.mInfo.InVoid || p.mInfo.Flying || p.mInfo.NoClip || (p.gamemode != packet.GameTypeSurvival && p.gamemode != packet.GameTypeAdventure) {
		p.mInfo.OnGround = false
		p.mInfo.ServerPosition = p.Position()
		p.mInfo.OldServerMovement = p.mInfo.ClientMovement
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
		p.mInfo.CanExempt = true
		exempt = true
	} else if !afk {
		exempt = p.mInfo.CanExempt
		p.aiStep()
		p.mInfo.CanExempt = false
		p.mInfo.LastBlocksSurrounding = surrounding
	}

	p.mInfo.Tick()
	if exempt {
		p.TryDebug(fmt.Sprintf("doMovementSimulation(): player exempted at frame %d", p.ClientFrame()), DebugTypeLogged, p.debugger.LogMovement)
	}

	p.validateMovement()
}

func (p *Player) updateMovementStates(pk *packet.PlayerAuthInput) {
	// Check if the player is in a loaded chunk, and if so, increment the tick counter.
	if p.inLoadedChunk {
		p.inLoadedChunkTicks++
	} else {
		p.inLoadedChunkTicks = 0
	}

	// Update the forward and left impulses of the player. This value is determined by the WASD combo the player
	// is holding. If on controller, this will be variable, depending on the joystick position.
	p.mInfo.ForwardImpulse = pk.MoveVector.Y() * 0.98
	p.mInfo.LeftImpulse = pk.MoveVector.X() * 0.98

	// Update the eye offset of the player - this is used in the attack position for combat validation.
	if p.mInfo.Sneaking {
		p.eyeOffset = 1.54
	} else {
		p.eyeOffset = 1.62
	}

	// Update movement key pressed states for the player, depending on what inputs the client has in it's input packet.
	p.mInfo.JumpBindPressed = utils.HasFlag(pk.InputData, packet.InputFlagJumpDown)
	p.mInfo.SprintBindPressed = utils.HasFlag(pk.InputData, packet.InputFlagSprintDown)
	p.mInfo.SneakBindPressed = utils.HasFlag(pk.InputData, packet.InputFlagSneakDown) || utils.HasFlag(pk.InputData, packet.InputFlagSneakToggleDown)

	// TODO: Make a better way to check if the player is in the void.
	p.mInfo.InVoid = p.Position().Y() < -128

	// Reset the jump velocity and gravity values in the players movement info, it will be updated later on.
	p.mInfo.JumpVelocity = game.DefaultJumpMotion
	p.mInfo.Gravity = game.NormalGravity

	// Update the effects of the player - this is ran after movement states are updated as some effects can
	// affect certain movement aspects, such as gravity.
	p.tickEffects()

	// Check the player's client-side flight status.
	if utils.HasFlag(pk.InputData, packet.InputFlagStartFlying) {
		p.mInfo.ToggleFly = true
		if p.mInfo.TrustFlyStatus {
			// If we don't trust the players flight status, we will have to wait for an update from the server.
			p.mInfo.Flying = true
		}
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopFlying) {
		p.mInfo.Flying = false
	}

	// Update the sneaking state of the player.
	if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) {
		p.mInfo.Sneaking = true
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
		p.mInfo.Sneaking = false
	}

	// Update the sprinting state of the player.
	var needsSpeedAdjustment bool
	startFlag, stopFlag := utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting), utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting)
	if startFlag && stopFlag {
		// When both the start and stop flags are found in the same tick, this usually indicates the player is horizontally collided as the client will
		// first check if the player is holding the sprint key (isn't sneaking, other conditions, etc.), and call setSprinting(true), but then see the player
		// is horizontally collided and call setSprinting(false) on the same call of onLivingUpdate()
		p.mInfo.Sprinting = false
		needsSpeedAdjustment = true
		p.mInfo.HasServerSpeedState = false
	} else if startFlag {
		p.mInfo.Sprinting = true
		needsSpeedAdjustment = true
		p.mInfo.HasServerSpeedState = false
	} else if stopFlag {
		p.mInfo.Sprinting = false
		// The client, if it has not recieved a speed state from the server, will set it's own speed (0.1). However, if the client stops sprinting
		// but has recieved a speed state from the server, it will wait for an update.
		needsSpeedAdjustment = !p.mInfo.HasServerSpeedState
	}

	// Estimate the client calculated speed of the player.
	// IMPORTANT: The client-side does not account for speed/slow effects in it's predicted speed.
	p.calculateClientSpeed()

	// If the player has switched sprinting state from false to true, adjust the movement speed
	// of the player to match the client calculated speed. It appears the client likes to do a
	// client-sided prediction of it's speed when enabling sprint, but not when stopping sprint.
	if needsSpeedAdjustment {
		p.mInfo.MovementSpeed = p.mInfo.ClientCalculatedSpeed
		p.TryDebug(fmt.Sprintf("updateMovementStates(): speed set to client calc @ %f", p.mInfo.MovementSpeed), DebugTypeLogged, p.debugger.LogMovement)
	}

	// Update the jumping state of the player.
	p.mInfo.Jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)
	if p.mInfo.JumpBindPressed {
		p.mInfo.TicksJumpBindHeld++
	} else {
		p.mInfo.TicksJumpBindHeld = 0
	}

	// Update the air speed of the player.
	p.mInfo.AirSpeed = 0.02
	if p.mInfo.Sprinting {
		p.mInfo.AirSpeed += 0.006
	}

	// If the player is not holding the jump key, reset the ticks until next jump.
	if !p.mInfo.JumpBindPressed {
		p.mInfo.TicksUntilNextJump = 0
	}

	// The client seems to send the ContainerClose packet late, so the client ends up moving around with the container *assumed* open.
	if p.containerOpen && (p.mInfo.ForwardImpulse > 0 || p.mInfo.LeftImpulse > 0 || p.mInfo.Jumping || p.mInfo.JumpBindPressed) {
		p.containerMoveTicks++
		if p.containerMoveTicks > 10 {
			p.Disconnect(game.ErrorInvalidInput)
		}
	} else if !p.containerOpen {
		p.containerMoveTicks = 0
	}
}

// clientCalculatedSpeed calculates the speed the client will be using before it recieves an update for it's movement speed.
func (p *Player) calculateClientSpeed() {
	// The base client calculated speed (no effects) is 0.1.
	p.mInfo.ClientCalculatedSpeed = 0.1

	if p.mInfo.ClientPredictsSpeed {
		if spd, ok := p.Effect(1); ok {
			p.mInfo.ClientCalculatedSpeed += float32(spd.Level()) * 0.02
		}

		if slw, ok := p.Effect(2); ok {
			p.mInfo.ClientCalculatedSpeed -= float32(slw.Level()) * 0.015
		}
	}

	// If the player is not sprinting, we don't need to multiply the
	// client calculated speed by 1.3.
	if !p.mInfo.Sprinting {
		return
	}

	p.mInfo.ClientCalculatedSpeed *= 1.3
}

// shiftTowardsClient shifts the server position towards the client position by a certain amount.
func (p *Player) shiftTowardsClient() {
	if p.mInfo.AwaitingCorrectionAcks > 0 || p.mInfo.StepClipOffset > 0 {
		return
	}

	shiftVec := mgl32.Vec3{p.mInfo.SupportedPositionPersuasion, 0, p.mInfo.SupportedPositionPersuasion}
	if !p.mInfo.InSupportedScenario {
		shiftVec = mgl32.Vec3{p.mInfo.UnsupportedPositionPersuasion, p.mInfo.UnsupportedPositionPersuasion, p.mInfo.UnsupportedPositionPersuasion}
	}

	// Shift the server position towards the client position by the acceptance amount.
	dPos := p.mInfo.ServerPosition.Sub(p.Position())
	p.mInfo.ServerPosition[0] -= game.ClampFloat(dPos.X(), -shiftVec.X(), shiftVec.X())
	p.mInfo.ServerPosition[1] -= game.ClampFloat(dPos.Y(), -shiftVec.Y(), shiftVec.Y())
	p.mInfo.ServerPosition[2] -= game.ClampFloat(dPos.Z(), -shiftVec.Z(), shiftVec.Z())

	dMov := p.mInfo.ServerMovement.Sub(p.mInfo.ClientPredictedMovement)
	p.mInfo.ServerMovement[0] -= game.ClampFloat(dMov.X(), -shiftVec.X(), shiftVec.X())
	p.mInfo.ServerMovement[1] -= game.ClampFloat(dMov.Y(), -shiftVec.Y(), shiftVec.Y())
	p.mInfo.ServerMovement[2] -= game.ClampFloat(dMov.Z(), -shiftVec.Z(), shiftVec.Z())
}

// validateMovement validates the movement of the player. If the position or the velocity of the player is offset by a certain amount, the player's movement will be corrected.
// If the player's position is within a certain range of the server's predicted position, then the server's position is set to the client's
func (p *Player) validateMovement() {
	if p.movementMode != utils.ModeFullAuthoritative {
		return
	}

	// Shift the position towards the client to allow for some leinancy.
	p.shiftTowardsClient()

	posDiff := p.mInfo.ServerPosition.Sub(p.Position())
	movDiff := p.mInfo.ServerMovement.Sub(p.mInfo.ClientPredictedMovement)
	p.TryDebug(fmt.Sprintf("validateMovement(): client pos:%v server pos:%v", p.Position(), p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
	p.TryDebug(fmt.Sprintf("validateMovement(): clientDelta:%v serverDelta:%v", p.mInfo.ClientPredictedMovement, p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)

	threshold := p.mInfo.MaxSupportedPositionDiff
	if !p.mInfo.InSupportedScenario {
		threshold = p.mInfo.MaxUnsupportedPositionDiff
	}
	threshold += p.mInfo.StepClipOffset
	p.TryDebug(fmt.Sprintf("validateMovement(): threshold=%f", threshold), DebugTypeLogged, p.debugger.LogMovement)

	if posDiff.LenSqr() <= threshold {
		return
	}
	p.TryDebug(fmt.Sprintf("validateMovement(): correction needed! posDiff=%v movDiff=%v", posDiff, movDiff), DebugTypeLogged, p.debugger.LogMovement)
	p.TryDebug(fmt.Sprintf("validateMovement(): correction sent for frame %d", p.ClientFrame()), DebugTypeChat, p.debugger.LogMovement)

	p.correctMovement()
}

// correctMovement sends a movement correction to the player. Exemptions can be made to prevent sending corrections, such as if
// the player has not received a correction yet, if the player is teleporting, or if the player is in an unsupported rewind scenario
// (determined by the people that made the rewind system) - in which case movement corrections will not work properly.
func (p *Player) correctMovement() {
	// Do not correct player movement if the movement mode is not fully server authoritative because it can lead to issues.
	if p.movementMode != utils.ModeFullAuthoritative {
		return
	}

	// Do not correct player movement if the player is in a scenario we cannot correct reliably.
	if p.mInfo.CanExempt {
		return
	}

	pos, delta := p.mInfo.ServerPosition, p.mInfo.ServerMovement

	// Send block updates for blocks around the player - to make sure that the world state
	// on the client is the same as the server's.
	if p.mInfo.TicksSinceBlockRefresh >= 10 {
		for bpos, b := range utils.GetNearbyBlocks(p.AABB(), true, true, p.World()) {
			p.conn.WritePacket(&packet.UpdateBlock{
				Position:          protocol.BlockPos{int32(bpos.X()), int32(bpos.Y()), int32(bpos.Z())},
				NewBlockRuntimeID: world.BlockRuntimeID(b),
				Flags:             packet.BlockUpdateNeighbours,
				Layer:             0,
			})
		}
		p.mInfo.TicksSinceBlockRefresh = 0
	}

	// This packet will correct the player's data, such as it's speed, etc.
	if p.lastAttributeData != nil {
		p.lastAttributeData.Tick = p.ClientFrame()
		p.conn.WritePacket(p.lastAttributeData)
	}

	// This packet will correct the player's states, such as sprinting, sneaking, etc.
	if p.lastActorData != nil {
		p.lastActorData.Tick = p.ClientFrame()
		p.conn.WritePacket(p.lastActorData)
	}

	// This packet will correct the player to the server's predicted position.
	p.conn.WritePacket(&packet.CorrectPlayerMovePrediction{
		Position: pos.Add(mgl32.Vec3{0, 1.62 + 1e-4}),
		Delta:    delta,
		OnGround: p.mInfo.OnGround,
		Tick:     p.ClientFrame(),
	})

	p.mInfo.AwaitingCorrectionAcks++
	p.Acknowledgement(func() {
		p.mInfo.AwaitingCorrectionAcks--
	})
}

// aiStep starts the movement simulation of the player.
func (p *Player) aiStep() {
	if !p.ready {
		p.mInfo.ServerMovement = mgl32.Vec3{}
		return
	}

	if p.mInfo.IsSmoothTeleport && p.mInfo.TicksSinceSmoothTeleport <= 3 {
		p.mInfo.ServerMovement = mgl32.Vec3{0, -0.078}

		delta := p.mInfo.TeleportPos.Sub(p.mInfo.ServerPosition)
		newPosRotInc := 3 - p.mInfo.TicksSinceSmoothTeleport
		if newPosRotInc != 0 {
			p.mInfo.ServerPosition = p.mInfo.ServerPosition.Add(delta.Mul(1 / float32(newPosRotInc)))
			p.TryDebug(fmt.Sprintf("aiStep(): smooth teleport newPosRotInc=%v, pos=%v", newPosRotInc, p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
			return
		}

		p.mInfo.ServerPosition = p.mInfo.TeleportPos
		p.mInfo.OnGround = p.mInfo.IsTeleportOnGround
		p.TryDebug(fmt.Sprintf("aiStep(): finishing smooth teleport to %v", p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
	}

	if p.mInfo.Teleporting && !p.mInfo.IsSmoothTeleport {
		p.mInfo.ServerPosition = p.mInfo.TeleportPos
		p.mInfo.ServerMovement = mgl32.Vec3{}
		if p.mInfo.OnGround {
			p.mInfo.ServerMovement[1] = -0.002
		}

		p.mInfo.TicksUntilNextJump = 0
		p.trySimulateJump()

		p.TryDebug(fmt.Sprintf("aiStep(): finished non-smooth teleport to %v w/ mov %v", p.mInfo.ServerPosition, p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)
		return
	}

	// Push the player out of any blocks they are inside of.
	p.pushOutOfBlock()

	// If the player's X movement is below 1e-7, set it to 0.
	if mgl32.Abs(p.mInfo.ServerMovement[0]) < 1e-7 {
		p.mInfo.ServerMovement[0] = 0
	}

	// If the player's Y movement is below 1e-7, set it to 0.
	if mgl32.Abs(p.mInfo.ServerMovement[1]) < 1e-7 {
		p.mInfo.ServerMovement[1] = 0
	}

	// If the player's Z movement is below 1e-7, set it to 0.
	if mgl32.Abs(p.mInfo.ServerMovement[2]) < 1e-7 {
		p.mInfo.ServerMovement[2] = 0
	}

	// If the player is not in a loaded chunk, or is immobile, set their movement to 0 vec.
	if p.mInfo.Immobile || !p.inLoadedChunk {
		p.mInfo.ServerMovement = mgl32.Vec3{}
		p.TryDebug("aiStep(): player immobile/not in loaded chunk", DebugTypeLogged, p.debugger.LogMovement)
		return
	}

	// Apply knockback if the server has sent it.
	if p.mInfo.TicksSinceKnockback == 0 {
		p.mInfo.ServerMovement = p.mInfo.Knockback
		p.TryDebug(fmt.Sprintf("aiStep(): knockback applied %v", p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)
	}

	p.trySimulateJump()
	p.simulateGroundMove()
}

// simulateGroundMove continues the player's movement simulation.
func (p *Player) simulateGroundMove() {
	if p.mInfo.StepClipOffset > 1e-7 {
		p.mInfo.StepClipOffset *= game.StepClipMultiplier
	} else {
		p.mInfo.StepClipOffset = 0
	}
	p.TryDebug(fmt.Sprintf("simulateGroundMove(): stepClipOffset=%f", p.mInfo.StepClipOffset), DebugTypeLogged, p.debugger.LogMovement)

	blockUnder := p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition.Sub(mgl32.Vec3{0, 0.5})))
	blockFriction := game.DefaultAirFriction

	if p.mInfo.OnGround {
		blockFriction *= utils.BlockFriction(blockUnder)
		p.TryDebug(fmt.Sprintf("simulateGroundMove(): block friction set to %f", blockFriction), DebugTypeLogged, p.debugger.LogMovement)
	}

	v3 := p.mInfo.getFrictionInfluencedSpeed(blockFriction)
	p.moveRelative(v3)

	nearClimableBlock := utils.BlockClimbable(p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition)))
	if nearClimableBlock && !p.mInfo.JumpBindPressed {
		p.mInfo.ServerMovement[0] = game.ClampFloat(p.mInfo.ServerMovement.X(), -0.2, 0.2)
		p.mInfo.ServerMovement[2] = game.ClampFloat(p.mInfo.ServerMovement.Z(), -0.2, 0.2)

		if p.mInfo.ServerMovement[1] < -0.2 {
			p.mInfo.ServerMovement[1] = -0.2
		}

		if p.mInfo.Sneaking && p.mInfo.ServerMovement.Y() < 0 {
			p.mInfo.ServerMovement[1] = 0
		}

		p.TryDebug(fmt.Sprintf("simulateGroundMove(): player near climbable, movement=%v", p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)
	}

	inCobweb := p.tryCobwebMovement()
	p.maybeBackOffFromEdge()

	oldMov := p.mInfo.ServerMovement
	p.simulateCollisions()

	isClimb := nearClimableBlock && (p.mInfo.HorizontallyCollided || p.mInfo.JumpBindPressed)
	if isClimb {
		p.mInfo.ServerMovement[1] = 0.2
		p.TryDebug("simulateGroundMove(): climb detected", DebugTypeLogged, p.debugger.LogMovement)
	}

	p.mInfo.ServerPosition = p.mInfo.ServerPosition.Add(p.mInfo.ServerMovement)
	p.TryDebug(fmt.Sprintf("simulateGroundMove(): final position=%v", p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)

	// Update `blockUnder` after collisions have been applied and the new position has been determined.
	blockUnder = p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition.Sub(mgl32.Vec3{0, 0.2})))
	if _, isAir := blockUnder.(block.Air); isAir {
		blockUnder2 := p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition).Side(cube.FaceDown))
		n := utils.BlockName(blockUnder2)
		if utils.IsFence(n) || utils.IsWall(n) || strings.Contains(n, "fence_gate") { // ask MCP
			blockUnder = blockUnder2
		}
	}

	// Check if there any collisions vertically/horizontally and then update the states in MovementInfo
	p.checkCollisions(oldMov, isClimb, blockUnder)

	// Simulate certain blocks that may modify the player's movement.
	if p.mInfo.OnGround && !p.mInfo.Sneaking {
		p.simulateStepOnBlock(blockUnder)
	}

	// If the player is in cobweb, we have to reset their movement to zero.
	if inCobweb {
		p.mInfo.ServerMovement = mgl32.Vec3{}
		p.TryDebug("simulateGroundMove(): in cobweb, mov set to 0 vec", DebugTypeLogged, p.debugger.LogMovement)
	}

	// If we cannot predict the movement scenario reliably, we trust the client's movement.
	if !p.isScenarioPredictable() {
		defer p.setMovementToClient()
	}

	p.mInfo.OldServerMovement = p.mInfo.ServerMovement

	p.simulateGravity()
	p.simulateHorizontalFriction(blockFriction)
	p.TryDebug(fmt.Sprintf("simulateGroundMove(): friction and gravity applied, movement=%v", p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)
}

func (p *Player) tryCobwebMovement() bool {
	// Check if the player is in a cobweb block.
	b, in := p.isInsideBlock()
	inCobweb := in && utils.BlockName(b) == "minecraft:web"
	if inCobweb {
		p.mInfo.ServerMovement[0] *= 0.25
		p.mInfo.ServerMovement[1] *= 0.05
		p.mInfo.ServerMovement[2] *= 0.25
		p.TryDebug(fmt.Sprintf("simulateGroundMove(): in cobweb, new mov=%v", p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)
	}

	return inCobweb
}

// simulateLandOnBlock simulates the player's movement when they fall and land on certain blocks.
func (p *Player) simulateLandOnBlock(oldMov mgl32.Vec3, b world.Block) bool {
	if !p.mInfo.OnGround || oldMov.Y() >= 0 || p.mInfo.SneakBindPressed {
		return false
	}
	handled := true

	switch utils.BlockName(b) {
	case "minecraft:slime":
		p.mInfo.ServerMovement[1] = oldMov.Y() * game.SlimeBounceMultiplier
		p.TryDebug(fmt.Sprintf("simulateFallOnBlock(): bounce on slime, new mov=%v", p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)
	case "minecraft:bed":
		p.mInfo.ServerMovement[1] = oldMov.Y() * game.BedBounceMultiplier
		if p.mInfo.ServerMovement[1] > 0.75 {
			p.mInfo.ServerMovement[1] = 0.75
		}

		p.TryDebug(fmt.Sprintf("simulateFallOnBlock(): bounce on bed, new mov=%v", p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)
	default:
		handled = false
	}

	return handled
}

// simulateStepOnBlock simulates the player's movement when they step on certain blocks.
func (p *Player) simulateStepOnBlock(b world.Block) {
	switch utils.BlockName(b) {
	case "minecraft:slime":
		yMov := math32.Abs(p.mInfo.ServerMovement.Y())
		if yMov < 0.1 && !p.mInfo.SneakBindPressed {
			d1 := 0.4 + yMov*0.2
			p.mInfo.ServerMovement[0] *= d1
			p.mInfo.ServerMovement[2] *= d1
		}

		if !p.debugger.LogMovement {
			return
		}

		p.Log().Debugf("simulateStepOnBlock(): walked on slime, new mov=%v", p.mInfo.ServerMovement)
	case "minecraft:soul_sand":
		p.mInfo.ServerMovement[0] *= 0.4
		p.mInfo.ServerMovement[2] *= 0.4
	}
}

// moveRelative simulates the additional movement force created by the player's mf/ms and rotation values
func (p *Player) moveRelative(fSpeed float32) {
	p.TryDebug(fmt.Sprintf("moveRelative(): fSpeed=%f", fSpeed), DebugTypeLogged, p.debugger.LogMovement)
	v := math32.Pow(p.mInfo.ForwardImpulse, 2) + math32.Pow(p.mInfo.LeftImpulse, 2)
	if v < 1e-4 {
		return
	}

	v = math32.Sqrt(v)
	if v < 1 {
		v = 1
	}

	v = fSpeed / v
	mf, ms := p.mInfo.ForwardImpulse*v, p.mInfo.LeftImpulse*v
	v2, v3 := game.MCSin(p.entity.Rotation().Z()*(math32.Pi/180)), game.MCCos(p.entity.Rotation().Z()*(math32.Pi/180))
	p.mInfo.ServerMovement[0] += ms*v3 - mf*v2
	p.mInfo.ServerMovement[2] += ms*v2 + mf*v3
}

// maybeBackOffFromEdge simulates the movement scenarios where a player is at the edge of a block.
func (p *Player) maybeBackOffFromEdge() {
	if !p.mInfo.Sneaking || !p.mInfo.OnGround || p.mInfo.ServerMovement.Y() > 0 {
		p.TryDebug("maybeBackOffFromEdge(): conditions not met.", DebugTypeLogged, p.debugger.LogMovement)
		return
	}

	currentVel := p.mInfo.ServerMovement
	bb := p.AABB().GrowVec3(mgl32.Vec3{-0.025, 0, -0.025})
	xMov, zMov, offset := currentVel.X(), currentVel.Z(), float32(0.05)

	for xMov != 0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -1.0, 0}), p.World())) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}
	}

	for zMov != 0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -1.0, zMov}), p.World())) == 0 {
		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	for xMov != 0 && zMov != 0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -1.0, zMov}), p.World())) == 0 {
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

	p.mInfo.ServerMovement[0] = xMov
	p.mInfo.ServerMovement[2] = zMov

	if !p.debugger.LogMovement {
		return
	}

	p.Log().Debugf("maybeBackOffFromEdge(): oldMov=%v, newMov=%v", currentVel, p.mInfo.ServerMovement)
}

// isInsideBlock returns true if the player is inside a block.
func (p *Player) isInsideBlock() (world.Block, bool) {
	bb := p.AABB()
	for pos, block := range utils.GetNearbyBlocks(bb, false, true, p.World()) {
		boxes := utils.BlockBoxes(block, pos, p.World())

		for _, box := range boxes {
			if !p.AABB().IntersectsWith(box.Translate(pos.Vec3())) {
				continue
			}

			p.TryDebug(fmt.Sprintf("isInsideBlock(): player inside block, block=%v", utils.BlockName(block)), DebugTypeLogged, p.debugger.LogMovement)
			return block, true
		}
	}

	return nil, false
}

// simulateCollisions simulates the player's collisions with blocks
func (p *Player) simulateCollisions() {
	currVel := p.mInfo.ServerMovement
	bbList := utils.GetNearbyBBoxes(p.AABB().Extend(currVel), p.World())
	newVel := currVel

	if currVel.LenSqr() > 0.0 {
		newVel = p.collideWithBlocks(currVel, p.AABB(), bbList)
		p.TryDebug(fmt.Sprintf("collide(): collideWithBlocks() result w/ oldVel=%v, newVel=%v", currVel, newVel), DebugTypeLogged, p.debugger.LogMovement)
	} else {
		p.TryDebug("collide(): currVel is 0 vector, collision not possible", DebugTypeLogged, p.debugger.LogMovement)
		return
	}

	xCollision := currVel[0] != newVel[0]
	yCollision := currVel[1] != newVel[1]
	zCollision := currVel[2] != newVel[2]
	hasGroundState := p.mInfo.OnGround || (yCollision && currVel[1] < 0.0)
	p.TryDebug(fmt.Sprintf("collide(): xCollision=%v, yCollision=%v, zCollision=%v, hasGroundState=%v", xCollision, yCollision, zCollision, hasGroundState), DebugTypeLogged, p.debugger.LogMovement)

	if hasGroundState && (xCollision || zCollision) {
		stepVel := mgl32.Vec3{currVel.X(), game.StepHeight, currVel.Z()}
		list := utils.GetNearbyBBoxes(p.AABB().Extend(stepVel), p.World())
		bb := p.AABB()

		bb, stepVel[1] = utils.DoBoxCollision(utils.CollisionY, bb, list, stepVel.Y())
		bb, stepVel[0] = utils.DoBoxCollision(utils.CollisionX, bb, list, stepVel.X())
		bb, stepVel[2] = utils.DoBoxCollision(utils.CollisionZ, bb, list, stepVel.Z())
		_, rDy := utils.DoBoxCollision(utils.CollisionY, bb, list, -(stepVel.Y()))
		stepVel[1] += rDy

		if game.Vec3HzDistSqr(newVel) < game.Vec3HzDistSqr(stepVel) {
			p.mInfo.StepClipOffset += stepVel.Y()
			newVel = stepVel
			p.TryDebug(fmt.Sprintf("collide(): step detected %v", stepVel), DebugTypeLogged, p.debugger.LogMovement)
		}
	}

	p.mInfo.ServerMovement = newVel
	p.TryDebug(fmt.Sprintf("collide(): movement=%v", newVel), DebugTypeLogged, p.debugger.LogMovement)
}

// collideWithBlocks simulates the player's collisions with blocks
func (p *Player) collideWithBlocks(vel mgl32.Vec3, bb cube.BBox, list []cube.BBox) mgl32.Vec3 {
	if len(list) == 0 {
		return vel
	}

	xMov, yMov, zMov := vel.X(), vel.Y(), vel.Z()
	if yMov != 0 {
		bb, yMov = utils.DoBoxCollision(utils.CollisionY, bb, list, yMov)
		p.TryDebug(fmt.Sprintf("collideWithBlocks(): oldYMov=%f, newYMov=%f", vel.Y(), yMov), DebugTypeLogged, p.debugger.LogMovement)
	}

	flag := math32.Abs(xMov) < math32.Abs(zMov)
	if flag && zMov != 0 {
		bb, zMov = utils.DoBoxCollision(utils.CollisionZ, bb, list, zMov)
		p.TryDebug(fmt.Sprintf("collideWithBlocks(): oldZMov=%f, newZMov=%f", vel.Z(), zMov), DebugTypeLogged, p.debugger.LogMovement)
	}

	if xMov != 0 {
		bb, xMov = utils.DoBoxCollision(utils.CollisionX, bb, list, xMov)
		p.TryDebug(fmt.Sprintf("collideWithBlocks(): oldXMov=%f, newXMov=%f", vel.X(), xMov), DebugTypeLogged, p.debugger.LogMovement)
	}

	if !flag && zMov != 0 {
		_, zMov = utils.DoBoxCollision(utils.CollisionZ, bb, list, zMov)
		p.TryDebug(fmt.Sprintf("collideWithBlocks(): oldZMov=%f, newZMov=%f", vel.Z(), zMov), DebugTypeLogged, p.debugger.LogMovement)
	}

	return mgl32.Vec3{xMov, yMov, zMov}
}

// pushOutOfBlock pushes the player outside of a block.
func (p *Player) pushOutOfBlock() {
	if p.mInfo.StepClipOffset > 0 {
		return
	}

	pos := cube.PosFromVec3(p.mInfo.ServerPosition)
	b, ba := p.World().GetBlock(pos), p.World().GetBlock(pos.Side(cube.FaceUp))

	if utils.CanPassBlock(b) {
		return
	}

	bb := p.AABB()
	for bpos, b := range utils.GetNearbyBlocks(bb, false, true, p.World()) {
		if utils.CanPassBlock(b) {
			continue
		}

		for _, box := range utils.BlockBoxes(b, bpos, p.World()) {
			box = box.Translate(bpos.Vec3())
			if box.Width() != 1 || box.Height() != 1 || box.Length() != 1 {
				continue
			}

			// The player is not inside the block's BB.
			if !bb.IntersectsWith(box) {
				continue
			}
			minDelta, maxDelta := box.Max().Sub(bb.Min()), box.Min().Sub(bb.Max())

			_, isAir := ba.(block.Air)
			if isAir && !p.mInfo.OnGround && box.Max().Y()-bb.Min().Y() > 0 && minDelta.Y() <= 0.5 {
				p.mInfo.ServerPosition[1] = box.Max().Y() + 1e-3
				p.TryDebug(fmt.Sprintf("pushOutOfBlocks(): push type 1 w/ new pos=%v", p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
				break
			}

			if bb.Max().X()-box.Min().X() > 0 && minDelta.X() <= 0.5 {
				p.mInfo.ServerPosition[0] = box.Max().X() + 0.5
				p.TryDebug(fmt.Sprintf("pushOutOfBlocks(): push type 2 w/ new pos=%v", p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
				break
			}

			if box.Max().X()-bb.Min().X() > 0 && maxDelta.X() >= -0.5 {
				p.mInfo.ServerPosition[0] = box.Min().X() - 0.5
				p.TryDebug(fmt.Sprintf("pushOutOfBlocks(): push type 3 w/ new pos=%v", p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
				break
			}

			if bb.Max().Z()-box.Min().Z() > 0 && minDelta.Z() <= 0.5 {
				p.mInfo.ServerPosition[2] = box.Max().Z() + 0.5
				p.TryDebug(fmt.Sprintf("pushOutOfBlocks(): push type 4 w/ new pos=%v", p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
				break
			}

			if box.Max().Z()-bb.Min().Z() > 0 && maxDelta.Z() >= -0.5 {
				p.mInfo.ServerPosition[2] = box.Min().Z() - 0.5
				p.TryDebug(fmt.Sprintf("pushOutOfBlocks(): push type 5 w/ new pos=%v", p.mInfo.ServerPosition), DebugTypeLogged, p.debugger.LogMovement)
				break
			}
		}
	}
}

// simulateGravity simulates the gravity of the player
func (p *Player) simulateGravity() {
	p.mInfo.ServerMovement[1] -= p.mInfo.Gravity
	p.mInfo.ServerMovement[1] *= game.GravityMultiplier
}

// simulateHorizontalFriction simulates the horizontal friction of the player
func (p *Player) simulateHorizontalFriction(friction float32) {
	p.mInfo.ServerMovement[0] *= friction
	p.mInfo.ServerMovement[2] *= friction
}

// trySimulateJump simulates the jump movement of the player
func (p *Player) trySimulateJump() {
	if !p.mInfo.Jumping {
		return
	}

	// If the player is not on the ground, we cannot simulate a jump.
	if !p.mInfo.OnGround {
		p.TryDebug("trySimulateJump#1: rejected jump, player not on ground", DebugTypeLogged, p.debugger.LogMovement)
		return
	}

	// If the player still has ticks remaining before they're allowed to jump, we cannot simulate this.
	if p.mInfo.TicksUntilNextJump > 0 {
		p.TryDebug(fmt.Sprintf("trySimulateJump#2: rejected jump, ticksUntilNextJump=%v", p.mInfo.TicksUntilNextJump), DebugTypeLogged, p.debugger.LogMovement)
		return
	}

	// Debug velocity after simulating as jump.
	defer p.TryDebug(fmt.Sprintf("trySimulateJump#3: jumpVel=%v", p.mInfo.ServerMovement), DebugTypeLogged, p.debugger.LogMovement)

	p.mInfo.ServerMovement[1] = p.mInfo.JumpVelocity
	p.mInfo.Jumping = true
	p.mInfo.TicksUntilNextJump = 10
	if !p.mInfo.Sprinting {
		return
	}

	force := p.entity.Rotation().Z() * 0.017453292
	p.mInfo.ServerMovement[0] -= game.MCSin(force) * 0.2
	p.mInfo.ServerMovement[2] += game.MCCos(force) * 0.2
}

// setMovementToClient sets the server's velocity and position to the client's
func (p *Player) setMovementToClient() {
	p.mInfo.ServerPosition = p.Position()
	p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
}

// checkCollisions compares the old and new velocities to check if there were any collisions made in p.collide()
func (p *Player) checkCollisions(old mgl32.Vec3, isClimb bool, b world.Block) {
	p.mInfo.XCollision = !mgl32.FloatEqualThreshold(old[0], p.mInfo.ServerMovement[0], 1e-5)
	p.mInfo.ZCollision = !mgl32.FloatEqualThreshold(old[2], p.mInfo.ServerMovement[2], 1e-5)

	p.mInfo.HorizontallyCollided = p.mInfo.XCollision || p.mInfo.ZCollision
	p.mInfo.VerticallyCollided = old[1] != p.mInfo.ServerMovement[1]
	p.mInfo.OnGround = p.mInfo.VerticallyCollided && old[1] < 0.0

	if isClimb {
		return
	}

	if p.mInfo.VerticallyCollided {
		if !p.simulateLandOnBlock(old, b) {
			p.mInfo.ServerMovement[1] = 0
		}

		p.TryDebug(fmt.Sprintf("checkCollisions(): collideY, onGround=%v", p.mInfo.OnGround), DebugTypeLogged, p.debugger.LogMovement)
	}

	if p.mInfo.XCollision {
		p.mInfo.ServerMovement[0] = 0
		p.TryDebug("checkCollisions(): collideX", DebugTypeLogged, p.debugger.LogMovement)
	}

	if p.mInfo.ZCollision {
		p.mInfo.ServerMovement[2] = 0
		p.TryDebug("checkCollisions(): collideZ", DebugTypeLogged, p.debugger.LogMovement)
	}
}

// isScenarioPredictable checks if the player is in an unsupported movement scenario.
// Returns false if the player is in a scenario we need to trust the client for.
func (p *Player) isScenarioPredictable() bool {
	bb := p.AABB()
	blocks := utils.GetNearbyBlocks(bb, false, false, p.World())

	p.mInfo.InSupportedScenario = true
	var hasLiquid, hasBamboo bool

	for _, bl := range blocks {
		_, ok := bl.(world.Liquid)
		if ok {
			hasLiquid = true
			continue
		}

		switch utils.BlockName(bl) {
		case "minecraft:bamboo":
			hasBamboo = true
		}
	}

	if hasLiquid || hasBamboo {
		p.TryDebug("isScenarioPredictable(): cannot predict scenario", DebugTypeLogged, p.debugger.LogMovement)
		return false
	}

	return true
}

type MovementInfo struct {
	CanExempt              bool
	ClientPredictsSpeed    bool
	AwaitingCorrectionAcks uint32

	ForwardImpulse float32
	LeftImpulse    float32

	JumpVelocity          float32
	AirSpeed              float32
	MovementSpeed         float32
	ClientCalculatedSpeed float32

	Gravity float32

	StepClipOffset float32

	MaxSupportedPositionDiff      float32
	MaxUnsupportedPositionDiff    float32
	SupportedPositionPersuasion   float32
	UnsupportedPositionPersuasion float32
	InSupportedScenario           bool

	TicksSinceKnockback      uint32
	TicksSinceBlockRefresh   uint32
	TicksSinceSmoothTeleport uint32
	TicksJumpBindHeld        uint32

	TicksUntilNextJump int32

	Sneaking, SneakBindPressed        bool
	Jumping, JumpBindPressed          bool
	Sprinting, SprintBindPressed      bool
	Immobile                          bool
	ToggleFly, Flying, TrustFlyStatus bool
	NoClip                            bool
	HasServerSpeedState               bool
	KnownInsideBlock                  bool

	IsCollided, VerticallyCollided, HorizontallyCollided bool
	XCollision, ZCollision                               bool
	OnGround                                             bool
	InVoid                                               bool

	Teleporting, AwaitingTeleport bool
	IsTeleportOnGround            bool
	IsSmoothTeleport              bool
	TeleportPos                   mgl32.Vec3

	ClientPredictedMovement, ClientMovement mgl32.Vec3
	Knockback                               mgl32.Vec3
	ServerMovement, OldServerMovement       mgl32.Vec3
	ServerPosition                          mgl32.Vec3

	LastBlocksSurrounding map[cube.Pos]world.Block
	LastUsedInput         *packet.PlayerAuthInput
}

// SetMaxPositionDeviations sets the amount of position deviation allowed, and if the client exceeds this,
// a correction will be sent.
func (m *MovementInfo) SetMaxPositionDeviations(a float32, b float32) {
	m.MaxSupportedPositionDiff = a * a
	m.MaxUnsupportedPositionDiff = b * b
}

// SetPositionPersuasions sets an amount the server position will be shifted toward the client.
func (m *MovementInfo) SetPositionPersuasions(a float32, b float32) {
	m.SupportedPositionPersuasion = a
	m.UnsupportedPositionPersuasion = b
}

// SetKnockback sets the knockback of the player.
func (m *MovementInfo) SetKnockback(k mgl32.Vec3) {
	m.Knockback = k
	m.TicksSinceKnockback = 0
}

// Tick ticks the movement info.
func (m *MovementInfo) Tick() {
	m.TicksSinceKnockback++
	m.TicksSinceBlockRefresh++
	m.TicksSinceSmoothTeleport++
	m.TicksJumpBindHeld++

	m.TicksUntilNextJump--
}

// getFrictionInfluencedSpeed returns the friction influenced speed of the player.
func (m *MovementInfo) getFrictionInfluencedSpeed(f float32) float32 {
	if m.OnGround {
		return m.MovementSpeed * (0.162771336 / (f * f * f))
	}

	return m.AirSpeed
}
