package simulation

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/oomph-ac/oomph/world"
	oomph_block "github.com/oomph-ac/oomph/world/block"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// SimulatePlayerMovement is a function that runs a movement simulation for
func SimulatePlayerMovement(p *player.Player) {
	movement := p.Movement()
	if movement == nil {
		p.Disconnect(game.ErrorInternalMissingMovementComponent)
		return
	}

	//assert.IsTrue(movement != nil, "movement component should be non-nil for simulation")

	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN movement sim for frame %d", p.SimulationFrame)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement sim for frame %d", p.SimulationFrame)

	if !simulationIsReliable(p) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: unsupported scenario", p.SimulationFrame)
		movement.Reset()
		return
	} else if p.World.GetChunk(protocol.ChunkPos{int32(movement.Pos().X()) >> 4, int32(movement.Pos().Z()) >> 4}) == nil {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: in unloaded chunk, cancelling all movement", p.SimulationFrame)
		movement.SetVel(mgl32.Vec3{})
		return
	}

	// We need to cap the velocity here to prevent Oomph from exhausting the server's resources.
	clampedVel := movement.Vel()
	clamped := false
	if math32.Abs(clampedVel.X()) > 10 {
		clamped = true
		clampedVel[0] = game.ClampFloat(clampedVel.X(), -10, 10)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "capped xVel to %f", clampedVel.X())
	}
	if math32.Abs(clampedVel.Y()) > 10 {
		clamped = true
		clampedVel[1] = game.ClampFloat(clampedVel.Y(), -10, 10)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "capped yVel to %f", clampedVel.Y())
	}
	if math32.Abs(clampedVel.Z()) > 10 {
		clamped = true
		clampedVel[2] = game.ClampFloat(clampedVel.Z(), -10, 10)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "capped zVel to %f", clampedVel.Z())
	}

	// Notify the player if their movement has been limited.
	if clamped {
		p.NMessage("<red>Movement limited to prevent server exhuastion.</red>")
		movement.SetVel(clampedVel)
	}

	blockUnder := p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.5}))))
	blockFriction := game.DefaultAirFriction

	// If a teleport was able to be handled, do not continue with the simulation.
	if attemptTeleport(movement) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", movement.Pos())
		if attemptKnockback(movement) {
			p.Dbg.Notify(player.DebugModeMovementSim, true, "knockback applied: %v", movement.Vel())
			movement.SetPos(movement.Pos().Add(movement.Vel()))
		}

		return
	}

	if movement.Immobile() || !p.Ready {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player is immobile")
		movement.SetVel(mgl32.Vec3{})
		return
	}

	// Reset the velocity to zero if it's significantly small.
	if movement.Vel().LenSqr() < 1e-12 {
		movement.SetVel(mgl32.Vec3{})
	}

	moveRelativeSpeed := movement.AirSpeed()
	if movement.OnGround() {
		blockFriction *= utils.BlockFriction(blockUnder)
		moveRelativeSpeed = movement.MovementSpeed() * (0.16277136 / (blockFriction * blockFriction * blockFriction))
	}

	if movement.Gliding() {
		_, hasElytra := p.Inventory().Chestplate().(item.Elytra)
		if hasElytra && !movement.OnGround() {
			movement.SetOnGround(false)
			simulateGlide(p, movement)
			movement.SetMov(movement.Vel())
		} else {
			if movement.OnGround() {
				movement.SetGliding(false)
			}
			p.Dbg.Notify(player.DebugModeMovementSim, true, "client wants glide, but has no elytra (or is on-ground) - forcing normal movement")
		}
		return
	} else {
		// Apply knockback if applicable.
		p.Dbg.Notify(player.DebugModeMovementSim, attemptKnockback(movement), "knockback applied: %v", movement.Vel())
		// Attempt jump velocity if applicable.
		p.Dbg.Notify(player.DebugModeMovementSim, attemptJump(movement), "jump force applied (sprint=%v): %v", movement.Sprinting(), movement.Vel())

		p.Dbg.Notify(player.DebugModeMovementSim, true, "blockUnder=%s, blockFriction=%v, speed=%v", utils.BlockName(blockUnder), blockFriction, moveRelativeSpeed)
		moveRelative(movement, moveRelativeSpeed)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "moveRelative force applied (vel=%v)", movement.Vel())

		nearClimable := utils.BlockClimbable(p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()))))
		if nearClimable && !movement.PressingJump() {
			newVel := movement.Vel()
			newVel[0] = game.ClampFloat(newVel[0], -0.2, 0.2)
			newVel[2] = game.ClampFloat(newVel[2], -0.2, 0.2)

			if newVel[1] < -0.2 {
				newVel[1] = -0.2
			}
			if movement.Sneaking() && newVel[1] < 0 {
				newVel[1] = 0
			}

			movement.SetVel(newVel)
		}

		blocksInside, isInsideBlock := blocksInside(movement, p.World)
		inCobweb := false
		if isInsideBlock {
			for _, b := range blocksInside {
				if utils.BlockName(b) == "minecraft:web" {
					inCobweb = true
					break
				}
			}
		}

		if inCobweb {
			newVel := movement.Vel()
			newVel[0] *= 0.25
			newVel[1] *= 0.05
			newVel[2] *= 0.25
			movement.SetVel(newVel)
			p.Dbg.Notify(player.DebugModeMovementSim, true, "cobweb force applied (vel=%v)", newVel)
		}

		// Avoid edges if the player is sneaking on the edge of a block.
		avoidEdge(movement, p.World)

		oldVel := movement.Vel()
		tryCollisions(movement, p.World, p.Dbg, p.VersionInRange(-1, player.GameVersion1_20_60))
		walkOnBlock(movement, blockUnder)
		movement.SetMov(movement.Vel())

		blockUnder = p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.2}))))
		if _, isAir := blockUnder.(block.Air); isAir {
			b := p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()).Side(cube.FaceDown)))
			if oomph_block.IsWall(b) || oomph_block.IsFence(b) {
				blockUnder = b
			}
		}

		isClimb := nearClimable && (p.Movement().XCollision() || p.Movement().ZCollision() || p.Movement().PressingJump())
		setPostCollisionMotion(movement, oldVel, isClimb, blockUnder)

		if inCobweb {
			p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move cobweb force applied (0 vel)")
			movement.SetVel(mgl32.Vec3{})
		}

		newVel := movement.Vel()
		newVel[1] -= movement.Gravity()
		newVel[1] *= game.GravityMultiplier
		newVel[0] *= blockFriction
		newVel[2] *= blockFriction

		if isClimb {
			newVel[1] = game.ClimbSpeed
			p.Dbg.Notify(player.DebugModeMovementSim, true, "upward climb applied")
		}

		movement.SetVel(newVel)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "serverPos=%v clientPos=%v, diff=%v", movement.Pos(), movement.Client().Pos(), movement.Pos().Sub(movement.Client().Pos()))
	}
}

func simulateGlide(p *player.Player, movement player.MovementComponent) {
	radians := (math32.Pi / 180.0)
	yaw, pitch := movement.Rotation().Z()*radians, movement.Rotation().X()*radians
	yawCos := game.MCCos(-yaw - math32.Pi)
	yawSin := game.MCSin(-yaw - math32.Pi)
	pitchCos := game.MCCos(pitch)
	pitchSin := game.MCSin(pitch)

	lookX := yawSin * -pitchCos
	lookY := -pitchSin
	lookZ := yawCos * -pitchCos

	vel := movement.Vel()
	velHz := math32.Sqrt(vel[0]*vel[0] + vel[2]*vel[2])
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

	// Although this should be applied when a fireworks entity is ticked (BECAUSE THIS IS WHAT EVERY SINGLE DECOMPILATION OF EVERY SINGLE
	// VERSION OF BOTH MCBE AND MCJE SHOWS), putting the logic here allowed Oomph's prediction to be accurate...
	// Furthermore, it seems that the client only likes to have one active boost at a time (although this seems to defy)
	// the logic provided by *EVERY SINGLE DECOMPILATION OF EVERY SINGLE VERSION OF BOTH MCBE AND MCJE*
	// Spending too many hours on stupid [sugar honey iced tea] like this better be making me bank in the near future.
	if movement.GlideBoost() > 0 {
		oldVel := vel
		vel[0] += (lookX * 0.1) + (((lookX * 1.5) - vel[0]) * 0.5)
		vel[1] += (lookY * 0.1) + (((lookY * 1.5) - vel[1]) * 0.5)
		vel[2] += (lookZ * 0.1) + (((lookZ * 1.5) - vel[2]) * 0.5)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "applied glide boost (old=%v new=%v)", oldVel, vel)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "glide boost dirVec=[%f %f %f]", lookX, lookY, lookZ)
	}

	vel[0] *= 0.99
	vel[1] *= 0.98
	vel[2] *= 0.99

	movement.SetVel(vel)

	oldVel := vel
	tryCollisions(movement, p.World, p.Dbg, p.VersionInRange(-1, player.GameVersion1_20_60))
	velDiff := movement.Vel().Sub(movement.Client().Vel())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "(glide) oldVel=%v, collisions=%v diff=%v", oldVel, movement.Vel(), velDiff)
}

func walkOnBlock(movement player.MovementComponent, blockUnder df_world.Block) {
	if !movement.OnGround() || movement.Sneaking() {
		return
	}

	newVel := movement.Vel()
	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		yMov := math32.Abs(newVel.Y())
		if yMov < 0.1 && !movement.PressingSneak() {
			d1 := 0.4 + yMov*0.2
			newVel[0] *= d1
			newVel[2] *= d1
		}
	case "minecraft:soul_sand":
		newVel[0] *= 0.3
		newVel[2] *= 0.3
	}
	movement.SetVel(newVel)
}

func simulationIsReliable(p *player.Player) bool {
	movement := p.Movement()
	if movement.RemainingTeleportTicks() > 0 {
		return true
	}

	for _, b := range utils.GetNearbyBlocks(movement.BoundingBox(), false, true, p.World) {
		if _, isLiquid := b.Block.(df_world.Liquid); isLiquid {
			return false
		}

		if utils.BlockName(b.Block) == "minecraft:bamboo" {
			return false
		}
	}

	return (p.GameMode == packet.GameTypeSurvival || p.GameMode == packet.GameTypeAdventure) &&
		!movement.Flying() && !movement.NoClip() && p.Alive
}

func landOnBlock(movement player.MovementComponent, old mgl32.Vec3, blockUnder df_world.Block) {
	newVel := movement.Vel()
	if movement.OnGround() || old.Y() >= 0 || movement.PressingSneak() {
		newVel[1] = 0
		movement.SetVel(newVel)
		return
	}

	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		newVel[1] = game.SlimeBounceMultiplier * old.Y()
	case "minecraft:bed":
		newVel[1] = game.BedBounceMultiplier * old.Y()
	default:
		newVel[1] = 0
	}
	movement.SetVel(newVel)
}

func setPostCollisionMotion(movement player.MovementComponent, old mgl32.Vec3, climb bool, blockUnder df_world.Block) {
	if climb {
		return
	}

	if movement.YCollision() {
		landOnBlock(movement, old, blockUnder)
	}

	newVel := movement.Vel()
	if movement.XCollision() {
		newVel[0] = 0
	}
	if movement.ZCollision() {
		newVel[2] = 0
	}
	movement.SetVel(newVel)
}

func tryCollisions(movement player.MovementComponent, w *world.World, dbg *player.Debugger, useSlideOffset bool) {
	var completedStep bool

	collisionBB := movement.BoundingBox()
	currVel := movement.Vel()
	bbList := utils.GetNearbyBBoxes(collisionBB.Extend(currVel), w)
	oneWayBlocks := utils.OneWayCollisionBlocks(utils.GetNearbyBlocks(collisionBB.Extend(currVel), false, false, w))

	// TODO: determine more blocks that are considered to be one-way physics blocks, and
	// figure out how to calculate ActorCollision::isStuckItem()
	useOneWayCollisions := len(oneWayBlocks) != 0 || movement.StuckInCollider()
	penetration := mgl32.Vec3{}
	dbg.Notify(player.DebugModeMovementSim, useOneWayCollisions, "one-way collisions are used for this simulation")

	yVel := mgl32.Vec3{0, currVel.Y()}
	xVel := mgl32.Vec3{currVel.X()}
	zVel := mgl32.Vec3{0, 0, currVel.Z()}

	for _, blockBox := range bbList {
		yVel = game.BBClipCollide(blockBox, collisionBB, yVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(mgl32.Vec3{0, yVel.Y()})
	dbg.Notify(player.DebugModeMovementSim, true, "Y-collision non-step=%f /w penetration=%f", yVel.Y(), penetration.Y())

	for _, blockBox := range bbList {
		xVel = game.BBClipCollide(blockBox, collisionBB, xVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(mgl32.Vec3{xVel.X()})
	dbg.Notify(player.DebugModeMovementSim, true, "X-collision non-step=%f /w penetration=%f", xVel.X(), penetration.X())

	for _, blockBox := range bbList {
		zVel = game.BBClipCollide(blockBox, collisionBB, zVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(mgl32.Vec3{0, 0, zVel.Z()})
	dbg.Notify(player.DebugModeMovementSim, true, "Z-collision non-step=%f /w penetration=%f", zVel.Z(), penetration.Z())

	collisionVel := mgl32.Vec3{xVel.X(), yVel.Y(), zVel.Z()}
	hasPenetration := penetration.LenSqr() >= 9.999999999999999e-12
	movement.SetStuckInCollider(movement.PenetratedLastFrame() && hasPenetration)
	movement.SetPenetratedLastFrame(hasPenetration)

	xCollision := currVel.X() != collisionVel.X()
	yCollision := currVel.Y() != collisionVel.Y()
	zCollision := currVel.Z() != collisionVel.Z()
	onGround := movement.OnGround() || (yCollision && currVel.Y() < 0.0)

	if onGround && (xCollision || zCollision) {
		yStepVel := mgl32.Vec3{0, game.StepHeight}
		xStepVel := mgl32.Vec3{currVel.X()}
		zStepVel := mgl32.Vec3{0, 0, currVel.Z()}

		stepBB := movement.BoundingBox()
		for _, blockBox := range bbList {
			yStepVel = game.BBClipCollide(blockBox, stepBB, yStepVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(yStepVel)

		for _, blockBox := range bbList {
			xStepVel = game.BBClipCollide(blockBox, stepBB, xStepVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(xStepVel)

		for _, blockBox := range bbList {
			zStepVel = game.BBClipCollide(blockBox, stepBB, zStepVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(zStepVel)

		inverseYStepVel := yStepVel.Mul(-1)
		for _, blockBox := range bbList {
			inverseYStepVel = game.BBClipCollide(blockBox, stepBB, inverseYStepVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(inverseYStepVel)
		yStepVel = yStepVel.Add(inverseYStepVel)

		stepVel := yStepVel.Add(zStepVel).Add(xStepVel)
		newBBList := utils.GetNearbyBBoxes(stepBB, w)
		dbg.Notify(player.DebugModeMovementSim, true, "newBBList count: %d", len(newBBList))
		dbg.Notify(player.DebugModeMovementSim, true, "stepVel=%v collisionVel=%v", stepVel, collisionVel)

		if len(newBBList) == 0 && game.Vec3HzDistSqr(collisionVel) < game.Vec3HzDistSqr(stepVel) {
			collisionVel = stepVel
			collisionBB = stepBB

			// This sliding offset is only used in versions 1.20.60 and below. On newer versions of the game, this sliding offset is no longer used.
			if useSlideOffset {
				completedStep = true
				slideOffset := movement.SlideOffset().Mul(game.SlideOffsetMultiplier)
				slideOffset[1] += stepVel.Y()
				//collisionVel[1] = currVel.Y()
				movement.SetSlideOffset(slideOffset)
			}

			dbg.Notify(player.DebugModeMovementSim, true, "step successful: %v", collisionVel)
		}
	}

	// We use the bounding box instead of oldPos.Add(collisionVel) to calculate the final position of the player because
	// it is accurate to vanilla's logic. Furthermore, it is useful such as in cases where the slide offset is being used
	// by older versions to calculate collisions.
	endPos := mgl32.Vec3{
		(collisionBB.Min().X() + collisionBB.Max().X()) / 2,
		collisionBB.Min().Y(),
		(collisionBB.Min().Z() + collisionBB.Max().Z()) / 2,
	}

	// useSlideOffset is true for any version that is below 1.20.70. For some reason, it seems that for versions above 1.20.60, the
	// slide offset is no longer used (confirmed via. testing w/ it).
	if useSlideOffset {
		if completedStep {
			// We don't add a debug message here, as it should already be noted in the statement where stepHeight is set
			endPos[1] -= movement.SlideOffset().Y()
			dbg.Notify(player.DebugModeMovementSim, true, "applying slideOffset, able to subtract endPos.y this frame by %f", movement.SlideOffset().Y())
		} else {
			// If there was no step done this tick, we can be certain that
			dbg.Notify(player.DebugModeMovementSim, true, "using slide offset, RESETTING slide offset vector")
			movement.SetSlideOffset(mgl32.Vec2{})
		}
	}
	movement.SetPos(endPos)

	yCollision = math32.Abs(currVel.Y()-collisionVel.Y()) >= 1e-5
	movement.SetCollisions(
		math32.Abs(currVel.X()-collisionVel.X()) >= 1e-5, // xCollision
		yCollision,
		math32.Abs(currVel.Z()-collisionVel.Z()) >= 1e-5, // zCollision
	)

	// Debug if the client doesn't have the same onGround state as our prediction.

	// Unlike Java, bedrock seems to have a strange condition for the client to be considered on the ground. This is probably useful
	// in cases where the client is teleporting, and the velocity (0) would still be equal to the previous velocity.
	movement.SetOnGround((yCollision && currVel.Y() < 0) || (movement.OnGround() && !yCollision && currVel.Y() == 0.0))
	movement.SetVel(collisionVel)

	dbg.Notify(player.DebugModeMovementSim, true, "finalVel=%v finalPos=%v", collisionVel, movement.Pos())
	dbg.Notify(player.DebugModeMovementSim, true, "clientVel=%v clientPos=%v", movement.Client().Mov(), movement.Client().Pos())
}

// avoidEdge is the function that helps the movement component remain at the edge of a block when sneaking.
func avoidEdge(movement player.MovementComponent, w *world.World) {
	if !movement.Sneaking() || !movement.OnGround() || movement.Vel().Y() > 0 {
		return
	}

	var (
		// Unlike in MCJE, where the edge boundry is 0.03, looking through a decomplilation of MCBE's 1.16 China
		// binary shows that on Bedrock the edge boundry is 0.025 on the X and Z axis.
		edgeBoundry float32 = 0.025
		offset      float32 = 0.05
	)

	newVel := movement.Vel()
	bb := movement.BoundingBox().GrowVec3(mgl32.Vec3{-edgeBoundry, 0, -edgeBoundry})
	xMov, zMov := newVel.X(), newVel.Z()

	for xMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, 0}), w)) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}
	}

	for zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -game.StepHeight * 1.01, zMov}), w)) == 0 {
		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	for xMov != 0.0 && zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, zMov}), w)) == 0 {
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
	movement.SetVel(newVel)
}

func blocksInside(movement player.MovementComponent, w *world.World) ([]df_world.Block, bool) {
	bb := movement.BoundingBox()
	blocks := []df_world.Block{}

	for _, result := range utils.GetNearbyBlocks(bb, false, true, w) {
		pos := result.Position
		block := result.Block
		boxes := utils.BlockCollisions(block, pos, w)

		for _, box := range boxes {
			if bb.IntersectsWith(box.Translate(pos.Vec3())) {
				blocks = append(blocks, block)
			}
		}
	}

	return blocks, len(blocks) > 0
}

func moveRelative(movement player.MovementComponent, moveRelativeSpeed float32) {
	impulse := movement.Impulse()
	force := impulse.Y()*impulse.Y() + impulse.X()*impulse.X()

	if force >= 1e-4 {
		force = moveRelativeSpeed / math32.Max(math32.Sqrt(force), 1.0)
		mf, ms := impulse.Y()*force, impulse.X()*force

		yaw := movement.Rotation().Z() * math32.Pi / 180.0
		v2, v3 := game.MCSin(yaw), game.MCCos(yaw)

		newVel := movement.Vel()
		newVel[0] += ms*v3 - mf*v2
		newVel[2] += mf*v3 + ms*v2
		movement.SetVel(newVel)
	}
}

func attemptKnockback(movement player.MovementComponent) bool {
	if movement.HasKnockback() {
		movement.SetVel(movement.Knockback())
		return true
	}
	return false
}

func attemptJump(movement player.MovementComponent) bool {
	if !movement.Jumping() || !movement.OnGround() || movement.JumpDelay() > 0 {
		return false
	} else if movement.SlideOffset().Y() > 0 && movement.Vel().Y() > 0 {
		return false
	}

	newVel := movement.Vel()
	newVel[1] = movement.JumpHeight()
	movement.SetJumpDelay(game.JumpDelayTicks)

	if movement.Sprinting() {
		force := movement.Rotation().Z() * 0.017453292
		newVel[0] -= game.MCSin(force) * 0.2
		newVel[2] += game.MCCos(force) * 0.2
	}

	movement.SetVel(newVel)
	return true
}

func attemptTeleport(movement player.MovementComponent) bool {
	if !movement.HasTeleport() {
		return false
	}

	if !movement.TeleportSmoothed() {
		movement.SetPos(movement.TeleportPos())
		movement.SetVel(mgl32.Vec3{})
		movement.SetJumpDelay(0)
		attemptJump(movement)
		return true
	}

	// Calculate the smooth teleport's next position.
	posDelta := movement.TeleportPos().Sub(movement.Pos())
	if remaining := movement.RemainingTeleportTicks() + 1; remaining > 0 {
		newPos := movement.Pos().Add(posDelta.Mul(1.0 / float32(remaining)))
		movement.SetPos(newPos)
		movement.SetVel(mgl32.Vec3{})
	}
	return false
}
