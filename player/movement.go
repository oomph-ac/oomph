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

	if p.debugger.LogMovement {
		p.Log().Debugf("%v started movement simulation for frame %d", p.Name(), p.ClientFrame())
		defer p.Log().Debugf("%v finished movement simulation for frame %d", p.Name(), p.ClientFrame())
	}

	if (p.movementMode == utils.ModeSemiAuthoritative && (p.inLoadedChunkTicks <= 5 || !p.ready)) || p.mInfo.InVoid || p.mInfo.Flying || p.mInfo.NoClip || (p.gamemode != packet.GameTypeSurvival && p.gamemode != packet.GameTypeAdventure) {
		p.mInfo.OnGround = false
		p.mInfo.ServerPosition = p.Position()
		p.mInfo.OldServerMovement = p.mInfo.ClientMovement
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
		p.mInfo.CanExempt = true
		exempt = true
	} else {
		exempt = p.mInfo.CanExempt
		p.aiStep()
		p.mInfo.CanExempt = false
	}

	p.mInfo.Tick()
	if exempt {
		p.Log().Debugf("doMovementSimulation(): player exempted at frame %d", p.ClientFrame())
		return
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

		if p.debugger.LogMovement {
			p.Log().Debugf("updateMovementStates(): speed set to client calc @ %f", p.mInfo.MovementSpeed)
		}
	}

	// Update the jumping state of the player.
	p.mInfo.Jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)

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

	// If the player is not sprinting, we don't need to multiply the
	// client calculated speed by 1.3.
	if !p.mInfo.Sprinting {
		return
	}

	p.mInfo.ClientCalculatedSpeed *= 1.3
}

// validateMovement validates the movement of the player. If the position or the velocity of the player is offset by a certain amount, the player's movement will be corrected.
// If the player's position is within a certain range of the server's predicted position, then the server's position is set to the client's
func (p *Player) validateMovement() {
	if p.movementMode != utils.ModeFullAuthoritative {
		return
	}

	posDiff := p.mInfo.ServerPosition.Sub(p.Position())
	movDiff := p.mInfo.ServerMovement.Sub(p.mInfo.ClientPredictedMovement)

	if p.debugger.LogMovement {
		p.Log().Debugf("validateMovement(): client pos:%v server pos:%v", p.Position(), p.mInfo.ServerPosition)
		p.Log().Debugf("validateMovement(): clientDelta:%v serverDelta:%v", p.mInfo.ClientPredictedMovement, p.mInfo.ServerMovement)
	}

	// TODO: Make Microjang fix shitty unsupported scenarios & fix my step code!!! -ethaniccc
	if posDiff.LenSqr() <= (p.mInfo.AcceptablePositionOffset + p.mInfo.UnsupportedAcceptance + math32.Pow(p.mInfo.StepClipOffset, 2)) {
		return
	}

	if p.debugger.LogMovement {
		p.Log().Debugf("validateMovement(): correction needed! posDiff=%f, movDiff=%f", posDiff, movDiff)
		p.SendOomphDebug("validateMovement(): correction sent for frame "+fmt.Sprint(p.ClientFrame()), packet.TextTypeChat)
	}

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
	if p.mInfo.TicksSinceBlockRefresh >= 40 {
		for bpos, b := range utils.GetNearbyBlocks(p.AABB(), true, p.World()) {
			p.conn.WritePacket(&packet.UpdateBlock{
				Position:          protocol.BlockPos{int32(bpos.X()), int32(bpos.Y()), int32(bpos.Z())},
				NewBlockRuntimeID: world.BlockRuntimeID(b),
				Flags:             packet.BlockUpdateNeighbours,
				Layer:             0,
			})
		}
		p.mInfo.TicksSinceBlockRefresh = 0
	}

	// This packet will correct the player to the server's predicted position.
	p.conn.WritePacket(&packet.CorrectPlayerMovePrediction{
		Position: pos.Add(mgl32.Vec3{0, 1.62 + 1e-4}),
		Delta:    delta,
		OnGround: p.mInfo.OnGround,
		Tick:     p.ClientFrame(),
	})
}

// aiStep starts the movement simulation of the player.
func (p *Player) aiStep() {
	if !p.ready {
		p.mInfo.ServerMovement = mgl32.Vec3{}
		return
	}

	if p.mInfo.Teleporting {
		p.mInfo.ServerMovement = mgl32.Vec3{}
		if p.mInfo.OnGround {
			p.mInfo.ServerMovement[1] = -0.002
		}

		p.mInfo.TicksUntilNextJump = 0
		if p.mInfo.Jumping {
			p.simulateJump()
		}

		return
	}

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

		if p.debugger.LogMovement {
			p.Log().Debug("aiStep(): player immobile/not in loaded chunk")
		}

		return
	}

	// Apply knockback if the server has sent it.
	if p.mInfo.TicksSinceKnockback == 0 {
		p.mInfo.ServerMovement = p.mInfo.Knockback

		if p.debugger.LogMovement {
			p.Log().Debugf("aiStep(): knockback applied %v", p.mInfo.ServerMovement)
		}
	}

	if p.mInfo.Jumping {
		if p.mInfo.OnGround && p.mInfo.TicksUntilNextJump <= 0 {
			p.simulateJump()
			if p.debugger.LogMovement {
				p.Log().Debug("aiStep(): simulated jump")
			}
		} else if p.debugger.LogMovement {
			p.Log().Debugf("aiStep(): refusing jump simulation (%v %v)", p.mInfo.OnGround, p.mInfo.TicksUntilNextJump)
		}
	}

	p.doGroundMove()
}

// doGroundMove continues the player's movement simulation.
func (p *Player) doGroundMove() {
	if p.mInfo.StepClipOffset > 0 {
		p.mInfo.StepClipOffset *= game.StepClipMultiplier
	}

	blockUnder := p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition.Sub(mgl32.Vec3{0, 0.5})))
	blockFriction := game.DefaultAirFriction

	if p.mInfo.OnGround {
		blockFriction *= utils.BlockFriction(blockUnder)

		if p.debugger.LogMovement {
			p.Log().Debugf("doGroundMove(): block friction set to %f", blockFriction)
		}
	}

	v3 := p.mInfo.getFrictionInfluencedSpeed(blockFriction)
	p.moveRelative(v3)

	nearClimableBlock := utils.BlockClimbable(p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition)))
	if nearClimableBlock {
		p.mInfo.ServerMovement[0] = game.ClampFloat(p.mInfo.ServerMovement.X(), -0.2, 0.2)
		p.mInfo.ServerMovement[2] = game.ClampFloat(p.mInfo.ServerMovement.Z(), -0.2, 0.2)

		if p.mInfo.ServerMovement[1] < -0.2 {
			p.mInfo.ServerMovement[1] = -0.2
		}

		if p.mInfo.Sneaking && p.mInfo.ServerMovement.Y() < 0 {
			p.mInfo.ServerMovement[1] = 0
		}

		if p.debugger.LogMovement {
			p.Log().Debugf("doGroundMove(): player near climbable, movement=%v", p.mInfo.ServerMovement)
		}
	}

	inCobweb := p.tryCobwebMovement()
	p.maybeBackOffFromEdge()

	oldMov := p.mInfo.ServerMovement
	p.simulateCollisions()

	p.mInfo.ServerPosition = p.mInfo.ServerPosition.Add(p.mInfo.ServerMovement)

	if p.debugger.LogMovement {
		p.Log().Debugf("doGroundMove(): final position=%v", p.mInfo.ServerPosition)
	}

	// Check if there any collisions vertically/horizontally and then update the states in MovementInfo
	p.checkCollisions(oldMov)

	// If the player is in cobweb, we have to reset their movement to zero.
	if inCobweb {
		p.mInfo.ServerMovement = mgl32.Vec3{}

		if p.debugger.LogMovement {
			p.Log().Debug("doGroundMove(): in cobweb, mov set to 0 vec")
		}
	}

	// Update `blockUnder` after collisions have been applied and the new position has been determined.
	blockUnder = p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition.Sub(mgl32.Vec3{0, 0.2})))
	if _, ok := blockUnder.(block.Air); ok {
		blockUnder2 := p.World().GetBlock(cube.PosFromVec3(p.mInfo.ServerPosition).Side(cube.FaceDown))
		n := utils.BlockName(blockUnder2)
		if utils.IsFence(n) || utils.IsWall(n) || strings.Contains(n, "fence_gate") { // ask MCP
			blockUnder = blockUnder2
		}
	}

	unsupported := p.checkUnsupportedMovementScenarios()
	if unsupported {
		defer p.setMovementToClient()
	}

	p.mInfo.OldServerMovement = p.mInfo.ServerMovement

	p.simulateGravity()
	p.simulateHorizontalFriction(blockFriction)

	if p.debugger.LogMovement {
		p.Log().Debugf("doGroundMove(): friction and gravity applied, movement=%v", p.mInfo.ServerMovement)
	}

	if p.mInfo.OnGround {
		p.checkFallState(oldMov, blockUnder)
	}

	if nearClimableBlock && (p.mInfo.HorizontallyCollided || p.mInfo.JumpBindPressed) {
		p.mInfo.ServerMovement[1] = 0.2

		if p.debugger.LogMovement {
			p.Log().Debug("doGroundMove(): climb detected at end frame")
		}
	}

	if p.mInfo.OnGround && !p.mInfo.Sneaking {
		p.simulateStepOnBlock(blockUnder)

		f := utils.BlockSpeedFactor(blockUnder)
		p.mInfo.ServerMovement[0] *= f
		p.mInfo.ServerMovement[2] *= f
	}

	// Attempt to push the player out of any blocks they're inside of.
	oldPos := p.mInfo.ServerPosition

	// TODO: Fix this.
	p.pushOutOfBlock()

	/* p.pushOutOfBlocks(p.mInfo.ServerPosition.X()-p.AABB().Width()*0.35, p.AABB().Min().Y()+0.5, p.mInfo.ServerPosition.Z()+p.AABB().Width()*0.35)
	p.pushOutOfBlocks(p.mInfo.ServerPosition.X()-p.AABB().Width()*0.35, p.AABB().Min().Y()+0.5, p.mInfo.ServerPosition.Z()-p.AABB().Width()*0.35)
	p.pushOutOfBlocks(p.mInfo.ServerPosition.X()+p.AABB().Width()*0.35, p.AABB().Min().Y()+0.5, p.mInfo.ServerPosition.Z()-p.AABB().Width()*0.35)
	p.pushOutOfBlocks(p.mInfo.ServerPosition.X()+p.AABB().Width()*0.35, p.AABB().Min().Y()+0.5, p.mInfo.ServerPosition.Z()+p.AABB().Width()*0.35) */

	if p.debugger.LogMovement && oldPos != p.mInfo.ServerPosition {
		p.Log().Debugf("doGroundMove(): pushed out of block results: old=%v, new=%v", oldPos, p.mInfo.ServerPosition)
	}
}

func (p *Player) tryCobwebMovement() bool {
	// Check if the player is in a cobweb block.
	b, in := p.isInsideBlock()
	inCobweb := in && utils.BlockName(b) == "minecraft:web"
	if inCobweb {
		p.mInfo.ServerMovement[0] *= 0.25
		p.mInfo.ServerMovement[1] *= 0.05
		p.mInfo.ServerMovement[2] *= 0.25

		if p.debugger.LogMovement {
			p.Log().Debugf("doGroundMove(): in cobweb, new mov=%v", p.mInfo.ServerMovement)
		}
	}

	return inCobweb
}

// checkFallState checks the falling state of the player and simulates the
// player's movement when they fall and land on certain blocks.
func (p *Player) checkFallState(oldMov mgl32.Vec3, b world.Block) {
	switch utils.BlockName(b) {
	case "minecraft:slime":
		if p.mInfo.SneakBindPressed {
			return
		}

		if oldMov.Y() >= 0 {
			return
		}

		// This is the closest we're probably going to get for now...
		p.mInfo.ServerMovement[1] = ((oldMov.Y() / game.GravityMultiplier) + p.mInfo.Gravity) * game.SlimeBounceMultiplier

		if !p.debugger.LogMovement {
			return
		}
		p.Log().Debugf("simulateFallOnBlock(): bounce on slime, new mov=%v", p.mInfo.ServerMovement)
	case "minecraft:bed":
		if p.mInfo.SneakBindPressed {
			return
		}

		if oldMov.Y() >= 0 {
			return
		}

		p.mInfo.ServerMovement[1] = ((oldMov.Y() / game.GravityMultiplier) + p.mInfo.Gravity) * game.BedBounceMultiplier

		if !p.debugger.LogMovement {
			return
		}
		p.Log().Debugf("simulateFallOnBlock(): bounce on bed, new mov=%v", p.mInfo.ServerMovement)
	}
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
	}
}

// moveRelative simulates the additional movement force created by the player's mf/ms and rotation values
func (p *Player) moveRelative(fSpeed float32) {
	if p.debugger.LogMovement {
		p.Log().Debugf("moveRelative(): fSpeed=%f", fSpeed)
	}

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
		if p.debugger.LogMovement {
			p.Log().Debug("maybeBackOffFromEdge(): conditions not met.")
		}

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
	if p.mInfo.StepClipOffset > 0 {
		return nil, false
	}

	bb := p.AABB()
	for pos, block := range utils.GetNearbyBlocks(bb, false, p.World()) {
		boxes := utils.BlockBoxes(block, pos, p.World())

		for _, box := range boxes {
			if !p.AABB().IntersectsWith(box.Translate(pos.Vec3())) {
				continue
			}

			if p.debugger.LogMovement {
				p.Log().Debugf("isInsideBlock(): player inside block, block=%v", utils.BlockName(block))
			}

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

		if p.debugger.LogMovement {
			p.Log().Debugf("collide(): collideWithBlocks() result w/ oldVel=%v, newVel=%v", currVel, newVel)
		}
	} else if p.debugger.LogMovement {
		p.Log().Debug("collide(): currVel is 0 vector, collision not possible")
	}

	xCollision := currVel[0] != newVel[0]
	yCollision := currVel[1] != newVel[1]
	zCollision := currVel[2] != newVel[2]
	hasGroundState := p.mInfo.OnGround || (yCollision && currVel[1] < 0.0)

	if p.debugger.LogMovement {
		p.Log().Debugf("collide(): xCollision=%v, yCollision=%v, zCollision=%v, hasGroundState=%v", xCollision, yCollision, zCollision, hasGroundState)
	}

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

			if p.debugger.LogMovement {
				p.Log().Debugf("collide(): conditions met to set movement to stepVel")
			}
		}
	}

	p.mInfo.ServerMovement = newVel

	if !p.debugger.LogMovement {
		return
	}

	p.Log().Debugf("collide(): movement=%v", newVel)
}

// collideWithBlocks simulates the player's collisions with blocks
func (p *Player) collideWithBlocks(vel mgl32.Vec3, bb cube.BBox, list []cube.BBox) mgl32.Vec3 {
	if len(list) == 0 {
		return vel
	}

	xMov, yMov, zMov := vel.X(), vel.Y(), vel.Z()
	if yMov != 0 {
		bb, yMov = utils.DoBoxCollision(utils.CollisionY, bb, list, yMov)

		if p.debugger.LogMovement {
			p.Log().Debugf("collideWithBlocks(): oldYMov=%f, newYMov=%f", vel.Y(), yMov)
		}
	}

	flag := math32.Abs(xMov) < math32.Abs(zMov)
	if flag && zMov != 0 {
		bb, zMov = utils.DoBoxCollision(utils.CollisionZ, bb, list, zMov)

		if p.debugger.LogMovement {
			p.Log().Debugf("collideWithBlocks(): oldZMov=%f, newZMov=%f", vel.Z(), zMov)
		}
	}

	if xMov != 0 {
		bb, xMov = utils.DoBoxCollision(utils.CollisionX, bb, list, xMov)

		if p.debugger.LogMovement {
			p.Log().Debugf("collideWithBlocks(): oldXMov=%f, newXMov=%f", vel.X(), xMov)
		}
	}

	if !flag && zMov != 0 {
		_, zMov = utils.DoBoxCollision(utils.CollisionZ, bb, list, zMov)

		if p.debugger.LogMovement {
			p.Log().Debugf("collideWithBlocks(): oldZMov=%f, newZMov=%f", vel.Z(), zMov)
		}
	}

	return mgl32.Vec3{xMov, yMov, zMov}
}

// pushOutOfBlock pushes the player outside of a block.
func (p *Player) pushOutOfBlock() {
	pos := cube.PosFromVec3(p.mInfo.ServerPosition)
	b, ba := p.World().GetBlock(pos), p.World().GetBlock(pos.Side(cube.FaceUp))

	var pushX, pushY, pushZ float32
	if utils.CanPassBlock(b) {
		return
	}

	bb := p.AABB()
	for bpos, block := range utils.GetNearbyBlocks(bb, false, p.World()) {
		for _, box := range utils.BlockBoxes(block, bpos, p.World()) {
			box = box.Translate(bpos.Vec3())
			alternateBB := box.GrowVec3(mgl32.Vec3{
				box.Width() * -0.5,
				box.Height() * -0.5,
				box.Width() * -0.5,
			})

			// In this case, the player is already too far inside the block's BB to be pushed out
			if bb.IntersectsWith(alternateBB) {
				p.SendOomphDebug("too far inside BB to be pushed out", 1)
				continue
			}

			if !bb.IntersectsWith(box) {
				p.SendOomphDebug("player is not intersecting with block", 1)
				continue
			}

			if p.mInfo.ServerPosition.X() > bb.Min().X() {
				pushX = box.Max().X() - bb.Min().X()
			} else if p.mInfo.ServerPosition.X() < bb.Max().X() {
				pushX = box.Min().X() - bb.Max().X()
			}

			if p.mInfo.ServerPosition.Y() > bb.Min().Y() {
				pushY = box.Max().Y() - bb.Min().Y()
			} else if p.mInfo.ServerPosition.Y() < bb.Max().Y() {
				pushY = box.Min().Y() - bb.Max().Y()
			}

			if p.mInfo.ServerPosition.Z() > bb.Min().Z() {
				pushZ = box.Max().Z() - bb.Min().Z()
			} else if p.mInfo.ServerPosition.Z() < bb.Max().Z() {
				pushZ = box.Min().Z() - bb.Max().Z()
			}
		}
	}

	if _, ok := ba.(block.Air); !ok {
		pushY = 0.0
	}

	fmt.Println(pushX, pushY, pushZ)
	p.mInfo.ServerPosition[0] += pushX
	p.mInfo.ServerPosition[1] += pushY
	p.mInfo.ServerPosition[2] += pushZ
}

// pushOutOfBlocks simulates the movement occured when the player is inside a block.
// @deprecated
func (p *Player) pushOutOfBlocks(x, y, z float32) {
	blockPos := cube.PosFromVec3(mgl32.Vec3{x, y, z})
	d0, d1, d2 := x-float32(blockPos.X()), y-float32(blockPos.Y()), z-float32(blockPos.Z())

	if utils.IsBlockOpenSpace(blockPos, p.World()) {
		if p.debugger.LogMovement {
			p.Log().Debugf("pushOutOfBlocks(): block at %v detected as full space, cannot continue", blockPos)
		}

		return
	}

	i := -1
	d3 := float32(9999.0)

	var pos cube.Pos

	if !utils.IsBlockFullCube(blockPos.Side(cube.FaceWest), p.World()) && d0 < d3 {
		d3 = d0
		i = 0
		pos = blockPos.Side(cube.FaceWest)
	}

	if !utils.IsBlockFullCube(blockPos.Side(cube.FaceEast), p.World()) && 1.0-d0 < d3 {
		d3 = 1.0 - d0
		i = 1
		pos = blockPos.Side(cube.FaceEast)
	}

	if !utils.IsBlockFullCube(blockPos.Side(cube.FaceDown), p.World()) && 1.0-d1 < d3 {
		d3 = 1.0 - d1
		i = 3
		pos = blockPos.Side(cube.FaceDown)
	} else if !utils.IsBlockFullCube(blockPos.Side(cube.FaceUp), p.World()) && 1.0-d1 < d3 {
		d3 = 1.0 - d1
		i = 2
		pos = blockPos.Side(cube.FaceUp)
	}

	if !utils.IsBlockFullCube(blockPos.Side(cube.FaceNorth), p.World()) && d2 < d3 {
		d3 = d2
		i = 4
		pos = blockPos.Side(cube.FaceNorth)
	}

	if !utils.IsBlockFullCube(blockPos.Side(cube.FaceSouth), p.World()) && 1.0-d2 < d3 {
		i = 5
		pos = blockPos.Side(cube.FaceSouth)
	}

	switch i {
	case 0:
		p.mInfo.ServerPosition[0] = float32(pos.X()) + 0.7
	case 1:
		p.mInfo.ServerPosition[0] = float32(pos.X()) + 0.3
	case 2:
		p.mInfo.ServerPosition[1] = float32(pos.Y())
	case 3:
		p.mInfo.ServerPosition[1] = float32(pos.Y())
	case 4:
		p.mInfo.ServerPosition[2] = float32(pos.Z()) + 0.7
	case 5:
		p.mInfo.ServerPosition[2] = float32(pos.Z()) + 0.3
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

// simulateJump simulates the jump movement of the player
func (p *Player) simulateJump() {
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
func (p *Player) checkCollisions(old mgl32.Vec3) {
	p.mInfo.XCollision = !mgl32.FloatEqualThreshold(old[0], p.mInfo.ServerMovement[0], 1e-5)
	p.mInfo.ZCollision = !mgl32.FloatEqualThreshold(old[2], p.mInfo.ServerMovement[2], 1e-5)

	p.mInfo.HorizontallyCollided = p.mInfo.XCollision || p.mInfo.ZCollision
	p.mInfo.VerticallyCollided = old[1] != p.mInfo.ServerMovement[1]
	p.mInfo.OnGround = p.mInfo.VerticallyCollided && old[1] < 0.0

	if p.mInfo.VerticallyCollided {
		p.mInfo.ServerMovement[1] = 0

		if p.debugger.LogMovement {
			p.Log().Debugf("checkCollisions(): collideY, onGround=%v", p.mInfo.OnGround)
		}
	}

	if p.mInfo.XCollision {
		p.mInfo.ServerMovement[0] = 0

		if p.debugger.LogMovement {
			p.Log().Debug("checkCollisions(): collideX")
		}
	}

	if p.mInfo.ZCollision {
		p.mInfo.ServerMovement[2] = 0

		if p.debugger.LogMovement {
			p.Log().Debug("checkCollisions(): collideZ")
		}
	}
}

// checkUnsupportedMovementScenarios checks if the player is in an unsupported movement scenario.
// Returns true if the player is in a scenario we cannot predict reliably.
func (p *Player) checkUnsupportedMovementScenarios() bool {
	bb := p.AABB()
	blocks := utils.GetNearbyBlocks(bb, false, p.World())

	p.mInfo.UnsupportedAcceptance = 0.0

	var hasLiquid, hasBounce bool

	for _, bl := range blocks {
		_, ok := bl.(world.Liquid)
		if ok {
			hasLiquid = true
			continue
		}

		switch utils.BlockName(bl) {
		case "minecraft:slime", "minecraft:bed":
			hasBounce = true
		}
	}

	if hasLiquid {
		if p.debugger.LogMovement {
			p.Log().Debug("checkUnsupportedMovementScenarios(): player in liquid")
		}

		return true
	}

	if hasBounce {
		p.mInfo.UnsupportedAcceptance += 1
	}

	if p.mInfo.UnsupportedAcceptance > 0 && p.debugger.LogMovement {
		p.Log().Debug("checkUnsupportedMovementScenarios(): player in unsupported rewind scenario")
	}

	return false
}

type MovementInfo struct {
	CanExempt bool

	ForwardImpulse float32
	LeftImpulse    float32

	JumpVelocity          float32
	AirSpeed              float32
	MovementSpeed         float32
	ClientCalculatedSpeed float32

	Gravity float32

	StepClipOffset           float32
	AcceptablePositionOffset float32
	UnsupportedAcceptance    float32

	TicksSinceKnockback    uint32
	TicksSinceBlockRefresh uint32
	TicksUntilNextJump     int32

	Sneaking, SneakBindPressed        bool
	Jumping, JumpBindPressed          bool
	Sprinting, SprintBindPressed      bool
	Teleporting, AwaitingTeleport     bool
	Immobile                          bool
	ToggleFly, Flying, TrustFlyStatus bool
	NoClip                            bool
	HasServerSpeedState               bool
	KnownInsideBlock                  bool

	IsCollided, VerticallyCollided, HorizontallyCollided bool
	XCollision, ZCollision                               bool
	OnGround                                             bool
	InVoid                                               bool

	ClientPredictedMovement, ClientMovement mgl32.Vec3
	Knockback                               mgl32.Vec3
	ServerMovement, OldServerMovement       mgl32.Vec3
	ServerPosition                          mgl32.Vec3

	LastUsedInput *packet.PlayerAuthInput
}

// SetAcceptablePositionOffset sets the acceptable position offset, and if the client exceeds this,
// the client will be sent a correction packet to correct it's movement.
func (m *MovementInfo) SetAcceptablePositionOffset(o float32) {
	m.AcceptablePositionOffset = o * o
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

	m.TicksUntilNextJump--
}

// getFrictionInfluencedSpeed returns the friction influenced speed of the player.
func (m *MovementInfo) getFrictionInfluencedSpeed(f float32) float32 {
	if m.OnGround {
		return m.MovementSpeed * (0.162771336 / (f * f * f))
	}

	return m.AirSpeed
}
