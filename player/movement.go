package player

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/ethaniccc/float32-cube/cube"

	"github.com/chewxy/math32"
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

	if p.inLoadedChunkTicks < 30 || !p.ready || p.mInfo.InVoid || p.mInfo.Flying || p.mInfo.NoClip {
		p.mInfo.OnGround = true
		p.mInfo.VerticallyCollided = true
		p.mInfo.ServerPosition = p.Position()
		p.mInfo.OldServerMovement = p.mInfo.ClientMovement
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
		p.mInfo.CanExempt = true
		exempt = true
	} else {
		p.mInfo.InUnsupportedRewindScenario = false
		exempt = p.mInfo.CanExempt
		p.aiStep()
		p.mInfo.CanExempt = false
	}

	p.mInfo.Tick()
	if exempt {
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

	// Update the sneaking state of the player.
	if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) {
		p.mInfo.Sneaking = true
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
		p.mInfo.Sneaking = false
	}

	// Update the sprinting state of the player.
	var needsSpeedAdjustment bool
	if utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting) && !p.mInfo.Sprinting {
		p.mInfo.Sprinting = true
		needsSpeedAdjustment = true
	} else if (utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting) || p.mInfo.ForwardImpulse <= 0 || p.mInfo.Sneaking) && p.mInfo.Sprinting {
		p.mInfo.Sprinting = false
	}

	// Estimate the client calculated speed of the player.
	p.calculateClientSpeed()

	// If the player has switched sprinting state from false to true, adjust the movement speed
	// of the player to match the client calculated speed. It appears the client likes to do a
	// client-sided prediction of it's speed when enabling sprint, but not when stopping sprint.
	if needsSpeedAdjustment {
		p.mInfo.MovementSpeed = p.mInfo.ClientCalculatedSpeed
	}

	// Update the jumping state of the player.
	p.mInfo.Jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)

	// Update the flying speed of the player.
	p.mInfo.FlyingSpeed = 0.02
	if p.mInfo.Sprinting {
		p.mInfo.FlyingSpeed += 0.006
	}

	// Apply knockback if neccessary.
	if p.mInfo.TicksSinceKnockback == 0 {
		p.mInfo.ServerMovement = p.mInfo.Knockback
	}

	// If the player is not holding the jump key, reset the ticks until next jump.
	if !p.mInfo.JumpBindPressed {
		p.mInfo.TicksUntilNextJump = 0
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
}

func (p *Player) calculateClientSpeed() {
	// The base client calculated speed (no effects) is 0.1.
	p.mInfo.ClientCalculatedSpeed = 0.1

	// Check if the slowness effect is present, and if so, adjust
	// the client calculated speed.
	if slwE, ok := p.effects[packet.EffectSlowness]; ok {
		p.mInfo.ClientCalculatedSpeed -= 0.015 * float32(slwE.Level())
	}

	// Check if the speed effect is present, and if so, adjust
	// the client calculated speed.
	if spdE, ok := p.effects[packet.EffectSpeed]; ok {
		p.mInfo.ClientCalculatedSpeed += 0.02 * float32(spdE.Level())
	}

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

	// TODO: Properly account for the client clipping into steps - as of now however,
	// this hack will suffice and will not trigger if a malicious client is stepping over
	// heights a vanilla client would not.
	if posDiff.LenSqr() <= (p.mInfo.AcceptablePositionOffset + math32.Pow(p.mInfo.StepClipOffset, 2)) {
		return
	}

	if p.debugger.Movement {
		p.SendOomphDebug(fmt.Sprint("got->", game.RoundVec32(p.Position(), 3), " want->", game.RoundVec32(p.mInfo.ServerPosition, 3), " clientMov->", game.RoundVec32(p.mInfo.ClientMovement, 4), " srvMov->", game.RoundVec32(p.mInfo.OldServerMovement, 4)), packet.TextTypeChat)
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
	if p.mInfo.CanExempt || p.mInfo.Teleporting || p.mInfo.InUnsupportedRewindScenario {
		return
	}

	pos, delta := p.mInfo.ServerPosition, p.mInfo.ServerMovement

	// Send block updates for blocks around the player - to make sure that the world state
	// on the client is the same as the server's.
	if p.mInfo.TicksSinceBlockRefresh >= 60 {
		for bpos, b := range p.GetNearbyBlocks(p.AABB()) {
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
		Position: pos.Add(mgl32.Vec3{0, 1.62 + 1e-3}),
		Delta:    delta,
		OnGround: p.mInfo.OnGround,
		Tick:     p.ClientFrame(),
	})
}

// aiStep starts the movement simulation of the player.
func (p *Player) aiStep() {
	if p.mInfo.Immobile || !p.inLoadedChunk {
		p.mInfo.ForwardImpulse = 0.0
		p.mInfo.LeftImpulse = 0.0
		p.mInfo.Jumping = false

		p.mInfo.ServerMovement = mgl32.Vec3{}
	}

	if p.mInfo.JumpBindPressed && p.mInfo.OnGround && p.mInfo.TicksUntilNextJump <= 0 {
		p.simulateJump()
		p.mInfo.TicksUntilNextJump = 10
	}

	p.doGroundMove()
}

// doGroundMove continues the player's movement simulation.
func (p *Player) doGroundMove() {
	if p.mInfo.StepClipOffset > 0 {
		p.mInfo.StepClipOffset *= game.StepClipMultiplier
	}

	blockFriction := game.DefaultAirFriction
	if p.mInfo.OnGround {
		if b, ok := p.Block(cube.PosFromVec3(p.mInfo.ServerPosition).Side(cube.FaceDown)).(block.Frictional); ok {
			blockFriction *= float32(b.Friction())
		} else {
			blockFriction *= 0.6
		}
	}

	v3 := p.mInfo.getFrictionInfluencedSpeed(blockFriction / game.DefaultAirFriction)
	p.moveRelative(v3)

	nearClimableBlock := utils.BlockClimbable(p.Block(cube.PosFromVec3(p.mInfo.ServerPosition)))
	if nearClimableBlock {
		p.mInfo.ServerMovement[0] = game.ClampFloat(p.mInfo.ServerMovement.X(), -0.2, 0.2)
		p.mInfo.ServerMovement[2] = game.ClampFloat(p.mInfo.ServerMovement.Z(), -0.2, 0.2)
		if p.mInfo.ServerMovement[1] < -0.2 {
			p.mInfo.ServerMovement[1] = -0.2
		}
		if p.mInfo.Sneaking && p.mInfo.ServerMovement.Y() < 0 {
			p.mInfo.ServerMovement[1] = 0
		}
	}

	p.maybeBackOffFromEdge()
	oldMov := p.mInfo.ServerMovement

	p.collide()
	p.mInfo.ServerPosition = p.mInfo.ServerPosition.Add(p.mInfo.ServerMovement)
	p.checkCollisions(oldMov)

	p.checkUnsupportedMovementScenarios()
	p.mInfo.OldServerMovement = p.mInfo.ServerMovement

	p.simulateGravity()
	p.simulateHorizontalFriction(blockFriction)

	if nearClimableBlock && (p.mInfo.HorizontallyCollided || p.mInfo.JumpBindPressed) {
		p.mInfo.ServerMovement[1] = 0.2
	}
}

// moveRelative simulates the additional movement force created by the player's mf/ms and rotation values
func (p *Player) moveRelative(fSpeed float32) {
	movVec := mgl32.Vec3{p.mInfo.LeftImpulse, 0, p.mInfo.ForwardImpulse}

	d0 := movVec.LenSqr()
	if d0 < 1e-7 {
		return
	}

	var newMovVec mgl32.Vec3
	if d0 > 1 {
		newMovVec = movVec.Normalize()
	} else {
		newMovVec = movVec
	}
	newMovVec = newMovVec.Mul(fSpeed)

	yaw := p.entity.Rotation().Z() * (math32.Pi / 180)
	v := game.MCSin(yaw)
	v1 := game.MCCos(yaw)

	p.mInfo.ServerMovement[0] += newMovVec.X()*v1 - newMovVec.Z()*v
	p.mInfo.ServerMovement[2] += newMovVec.Z()*v1 + newMovVec.X()*v
}

// maybeBackOffFromEdge simulates the movement scenarios where a player is at the edge of a block.
func (p *Player) maybeBackOffFromEdge() {
	if !p.mInfo.Sneaking || !p.mInfo.OnGround || p.mInfo.ServerMovement[1] > 0 {
		return
	}

	currentVel := p.mInfo.ServerMovement
	if currentVel[1] > 0 {
		return
	}

	bb := p.AABB()
	d0, d1, d2 := currentVel.X(), currentVel.Z(), float32(0.05)

	for d0 != 0 && len(p.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{d0, -game.StepHeight, 0}))) == 0 {
		if d0 < d2 && d0 >= -d2 {
			d0 = 0
		} else if d0 > 0 {
			d0 -= d2
		} else {
			d0 += d2
		}
	}

	for d1 != 0 && len(p.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -game.StepHeight, d1}))) == 0 {
		if d1 < d2 && d1 >= -d2 {
			d1 = 0
		} else if d1 > 0 {
			d1 -= d2
		} else {
			d1 += d2
		}
	}

	for d0 != 0 && d1 != 0 && len(p.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{d0, -game.StepHeight, d1}))) == 0 {
		if d0 < d2 && d0 >= -d2 {
			d0 = 0
		} else if d0 > 0 {
			d0 -= d2
		} else {
			d0 += d2
		}

		if d1 < d2 && d1 >= -d2 {
			d1 = 0
		} else if d1 > 0 {
			d1 -= d2
		} else {
			d1 += d2
		}
	}

	p.mInfo.ServerMovement = mgl32.Vec3{d0, currentVel.Y(), d1}
}

// collide simulates the player's collisions with blocks
func (p *Player) collide() {
	currVel := p.mInfo.ServerMovement
	bbList := p.GetNearbyBBoxes(p.AABB().Extend(currVel))
	newVel := currVel

	if currVel.LenSqr() > 0.0 {
		newVel = p.collideWithBlocks(currVel, p.AABB(), bbList)
	}

	xCollision := currVel[0] != newVel[0]
	yCollision := currVel[1] != newVel[1]
	zCollision := currVel[2] != newVel[2]
	hasGroundState := p.mInfo.OnGround || (yCollision && currVel[1] < 0.0)

	if hasGroundState && (xCollision || zCollision) {
		stepVel := mgl32.Vec3{currVel.X(), game.StepHeight, currVel.Z()}
		list := p.GetNearbyBBoxes(p.AABB().Extend(stepVel))

		bb := p.AABB()
		bb, stepVel[1] = utils.DoBoxCollision(utils.CollisionY, bb, list, stepVel.Y())
		bb, stepVel[0] = utils.DoBoxCollision(utils.CollisionX, bb, list, stepVel.X())
		bb, stepVel[2] = utils.DoBoxCollision(utils.CollisionZ, bb, list, stepVel.Z())
		_, rDy := utils.DoBoxCollision(utils.CollisionY, bb, list, -(stepVel.Y()))
		stepVel[1] += rDy

		if game.Vec3HzDistSqr(newVel) < game.Vec3HzDistSqr(stepVel) {
			p.mInfo.StepClipOffset += stepVel.Y()
			newVel = stepVel
		}
	}

	p.mInfo.ServerMovement = newVel
}

// collideWithBlocks simulates the player's collisions with blocks
func (p *Player) collideWithBlocks(vel mgl32.Vec3, bb cube.BBox, list []cube.BBox) mgl32.Vec3 {
	if len(list) == 0 {
		return vel
	}

	xMov, yMov, zMov := vel.X(), vel.Y(), vel.Z()
	if yMov != 0 {
		bb, yMov = utils.DoBoxCollision(utils.CollisionY, bb, list, yMov)
	}

	flag := math32.Abs(xMov) < math32.Abs(zMov)
	if flag && zMov != 0 {
		bb, zMov = utils.DoBoxCollision(utils.CollisionZ, bb, list, zMov)
	}

	if xMov != 0 {
		bb, xMov = utils.DoBoxCollision(utils.CollisionX, bb, list, xMov)
	}

	if !flag && zMov != 0 {
		_, zMov = utils.DoBoxCollision(utils.CollisionZ, bb, list, zMov)
	}

	return mgl32.Vec3{xMov, yMov, zMov}
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
	}

	if p.mInfo.XCollision {
		p.mInfo.ServerMovement[0] = 0
	}

	if p.mInfo.ZCollision {
		p.mInfo.ServerMovement[2] = 0
	}
}

// checkUnsupportedMovementScenarios checks if the player is in an unsupported movement scenario
func (p *Player) checkUnsupportedMovementScenarios() {
	bb := p.AABB()
	blocks := p.GetNearbyBlocks(bb)

	/* The following checks below determine wether or not the player is in an unspported rewind scenario.
	What this means is that the movement corrections on the client won't work properly and the player will
	essentially be jerked around indefinently, and therefore, corrections should not be done if these conditions
	are met. */

	// This check determines if the player is inside any blocks
	/* if cube.AnyIntersections(boxes, bb) && !p.mInfo.HorizontallyCollided && !p.mInfo.VerticallyCollided {
		p.mInfo.InUnsupportedRewindScenario = true
	} */

	// This check determines if the player is near liquids
	for _, bl := range blocks {
		_, ok := bl.(world.Liquid)
		if ok {
			p.mInfo.InUnsupportedRewindScenario = true
			break
		}
	}

	if p.mInfo.InUnsupportedRewindScenario {
		p.setMovementToClient()
	}
}

type MovementInfo struct {
	CanExempt                   bool
	InUnsupportedRewindScenario bool

	ForwardImpulse float32
	LeftImpulse    float32

	JumpVelocity          float32
	FlyingSpeed           float32
	MovementSpeed         float32
	ClientCalculatedSpeed float32

	Gravity float32

	StepClipOffset           float32
	AcceptablePositionOffset float32

	TicksSinceKnockback    uint32
	TicksSinceBlockRefresh uint32
	TicksUntilNextJump     int32

	Sneaking, SneakBindPressed   bool
	Jumping, JumpBindPressed     bool
	Sprinting, SprintBindPressed bool
	Teleporting                  bool
	Immobile                     bool
	CanFly, Flying               bool
	NoClip                       bool

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
		return m.MovementSpeed * (0.21600002 / (f * f * f))
	}

	return m.FlyingSpeed
}
