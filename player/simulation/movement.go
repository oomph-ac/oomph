package simulation

import (
	"math"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	oomph_block "github.com/oomph-ac/oomph/world/block"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// SimulatePlayerMovement is a function that runs a movement simulation for
func SimulatePlayerMovement(p *player.Player, movement player.MovementComponent) {
	if movement == nil {
		p.Disconnect(game.ErrorInternalMissingMovementComponent)
		return
	}

	//assert.IsTrue(movement != nil, "movement component should be non-nil for simulation")

	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN movement sim for frame %d", p.SimulationFrame)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement sim for frame %d", p.SimulationFrame)

	p.Dbg.Notify(player.DebugModeMovementSim, true, "mF=%.4f, mS=%.4f", movement.Impulse().Y(), movement.Impulse().X())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "yaw=%.4f, pitch=%.4f", movement.Rotation().Z(), movement.Rotation().X())

	// ALWAYS simulate the teleport, as the client will always have the same behavior regardless of if the scenario
	// is "unreliable", or if the player currently is in an unloaded chunk.
	if attemptTeleport(p, p.Dbg) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", movement.Pos())
		return
	}

	if !simulationIsReliable(p, movement) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: unsupported scenario", p.SimulationFrame)
		movement.Reset()
		return
	} else if p.World().GetChunk(protocol.ChunkPos{int32(movement.Pos().X()) >> 4, int32(movement.Pos().Z()) >> 4}) == nil {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: in unloaded chunk, cancelling all movement", p.SimulationFrame)
		movement.SetVel(mgl32.Vec3{})
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

	blockUnder := p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.5}))))
	blockFriction := game.DefaultAirFriction
	moveRelativeSpeed := movement.AirSpeed()
	if movement.OnGround() {
		mSpeed := movement.MovementSpeed()
		if utils.BlockName(blockUnder) == "minecraft:soul_sand" {
			mSpeed *= 0.543
		}
		blockFriction *= utils.BlockFriction(blockUnder)
		moveRelativeSpeed = mSpeed * (0.16277136 / (blockFriction * blockFriction * blockFriction))
	}

	if movement.Gliding() {
		_, hasElytra := p.Inventory().Chestplate().Item().(item.Elytra)
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
		var clientJumpPrevented bool

		// Apply knockback if applicable.
		p.Dbg.Notify(player.DebugModeMovementSim, attemptKnockback(movement), "knockback applied: %v", movement.Vel())
		// Attempt jump velocity if applicable.
		p.Dbg.Notify(player.DebugModeMovementSim, true, "blockUnder=%s, blockFriction=%v, speed=%v", utils.BlockName(blockUnder), blockFriction, moveRelativeSpeed)
		moveRelative(movement, moveRelativeSpeed)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "moveRelative force applied (vel=%v)", movement.Vel())
		p.Dbg.Notify(player.DebugModeMovementSim, attemptJump(p, p.Dbg, &clientJumpPrevented), "jump force applied (sprint=%v): %v", movement.Sprinting(), movement.Vel())

		nearClimbable := utils.BlockClimbable(p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()))))
		if nearClimbable {
			newVel := movement.Vel()
			//newVel[0] = game.ClampFloat(newVel[0], -0.3, 0.3)
			//newVel[2] = game.ClampFloat(newVel[2], -0.3, 0.3)

			negClimbSpeed := -game.ClimbSpeed
			if newVel[1] < negClimbSpeed {
				newVel[1] = negClimbSpeed
			}
			if movement.PressingJump() || movement.XCollision() || movement.ZCollision() {
				newVel[1] = game.ClimbSpeed
			}
			if movement.Sneaking() && newVel[1] < 0 {
				newVel[1] = 0
			}

			p.Dbg.Notify(player.DebugModeMovementSim, true, "added climb velocity: %v (collided=%v pressingJump=%v)", newVel, movement.XCollision() || movement.ZCollision(), movement.PressingJump())
			movement.SetVel(newVel)
		}

		blocksInside, isInsideBlock := blocksInside(movement, p.World())
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
		avoidEdge(movement, p.World(), p.Dbg)

		oldVel := movement.Vel()
		oldOnGround := movement.OnGround()
		oldY := movement.Pos().Y()

		tryCollisions(p, p.World(), p.Dbg, p.VersionInRange(-1, player.GameVersion1_20_60), clientJumpPrevented)
		if supportPos := movement.SupportingBlockPos(); supportPos != nil {
			blockUnder = p.World().Block([3]int(*supportPos))
		} else {
			blockUnder = p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.2}))))
			if _, isAir := blockUnder.(block.Air); isAir {
				b := p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()).Side(cube.FaceDown)))
				if oomph_block.IsWall(b) || oomph_block.IsFence(b) {
					blockUnder = b
				}
			}
		}

		if oldY == movement.Pos().Y() {
			walkOnBlock(movement, p.Dbg, blockUnder)
		} else {
			p.Dbg.Notify(player.DebugModeMovementSim, true, "NO WALKING ON BLOCK FUCK YOU")
		}

		movement.SetMov(movement.Vel())
		setPostCollisionMotion(p, oldVel, oldOnGround, blockUnder)

		if inCobweb {
			p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move cobweb force applied (0 vel)")
			movement.SetVel(mgl32.Vec3{})
		}

		newVel := movement.Vel()
		if eff, ok := p.Effects().Get(packet.EffectLevitation); ok {
			levSpeed := game.LevitationGravityMultiplier * float32(eff.Amplifier)
			newVel[1] += (levSpeed - newVel[1]) * 0.2
		} else {
			newVel[1] -= movement.Gravity()
			newVel[1] *= game.NormalGravityMultiplier
		}
		newVel[0] *= blockFriction
		newVel[2] *= blockFriction

		movement.SetVel(newVel)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "endOfFrameVel=%v", newVel)
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
	tryCollisions(p, p.World(), p.Dbg, p.VersionInRange(-1, player.GameVersion1_20_60), false)
	velDiff := movement.Vel().Sub(movement.Client().Vel())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "(glide) oldVel=%v, collisions=%v diff=%v", oldVel, movement.Vel(), velDiff)
}

func walkOnBlock(movement player.MovementComponent, dbg *player.Debugger, blockUnder world.Block) {
	if !movement.OnGround() || movement.Sneaking() {
		dbg.Notify(player.DebugModeMovementSim, true, "walkOnBlock: conditions not met (onGround=%v sneaking=%v)", movement.OnGround(), movement.Sneaking())
		return
	}

	oldVel := movement.Vel()
	newVel := movement.Vel()
	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		yMov := math32.Abs(newVel.Y())
		if yMov < 0.1 && !movement.PressingSneak() {
			d1 := 0.4 + yMov*0.2
			newVel[0] *= d1
			newVel[2] *= d1
		}
	}
	dbg.Notify(player.DebugModeMovementSim, true, "walkOnBlock: oldVel=%v newVel=%v", oldVel, newVel)
	movement.SetVel(newVel)
}

func simulationIsReliable(p *player.Player, movement player.MovementComponent) bool {
	if movement.RemainingTeleportTicks() > 0 {
		return true
	}

	for _, b := range utils.GetNearbyBlocks(movement.BoundingBox().Grow(1), false, true, p.World()) {
		if _, isLiquid := b.Block.(world.Liquid); isLiquid {
			blockBB := cube.Box(0, 0, 0, 1, 1, 1).Translate(b.Position.Vec3())
			if movement.BoundingBox().IntersectsWith(blockBB) {
				return false
			}
		}
		if utils.BlockName(b.Block) == "minecraft:bamboo" {
			return false
		}
	}

	return (p.GameMode == packet.GameTypeSurvival || p.GameMode == packet.GameTypeAdventure) &&
		!(movement.Flying() || movement.JustDisabledFlight() || movement.NoClip() || !p.Alive)
}

func landOnBlock(movement player.MovementComponent, old mgl32.Vec3, blockUnder world.Block) {
	newVel := movement.Vel()
	if old.Y() >= 0 || movement.PressingSneak() {
		newVel[1] = 0
		movement.SetVel(newVel)
		return
	}

	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		newVel[1] = game.SlimeBounceMultiplier * old.Y()
		if math32.Abs(newVel[1]) < 1e-4 {
			newVel[1] = 0.0
		}
	case "minecraft:bed":
		newVel[1] = math32.Min(1.0, game.BedBounceMultiplier*old.Y())
	default:
		newVel[1] = 0
	}
	movement.SetVel(newVel)
}

func setPostCollisionMotion(p *player.Player, oldVel mgl32.Vec3, oldOnGround bool, blockUnder world.Block) {
	movement := p.Movement()
	if !oldOnGround && movement.YCollision() {
		landOnBlock(movement, oldVel, blockUnder)
	} else if movement.YCollision() {
		newVel := movement.Vel()
		newVel[1] = 0
		movement.SetVel(newVel)
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

func isJumpBlocked(p *player.Player, jumpVel mgl32.Vec3) bool {
	movement := p.Movement()
	collisionBB := movement.BoundingBox()
	bbList := utils.GetNearbyBBoxes(collisionBB.Extend(jumpVel), p.World())

	yVel := mgl32.Vec3{0, jumpVel.Y()}
	xVel := mgl32.Vec3{jumpVel.X()}
	zVel := mgl32.Vec3{0, 0, jumpVel.Z()}

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		yVel = game.BBClipCollide(blockBox, collisionBB, yVel, false, nil)
	}
	collisionBB = collisionBB.Translate(yVel)

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		xVel = game.BBClipCollide(blockBox, collisionBB, xVel, false, nil)
	}
	collisionBB = collisionBB.Translate(xVel)

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		zVel = game.BBClipCollide(blockBox, collisionBB, zVel, false, nil)
	}
	initalBlockCond := ((xVel[0] != jumpVel[0]) || (zVel[2] != jumpVel[2])) && yVel[1] == jumpVel[1]
	if !initalBlockCond {
		return false
	}

	xVel = mgl32.Vec3{jumpVel.X()}
	yVel = mgl32.Vec3{0, jumpVel.Y()}
	zVel = mgl32.Vec3{0, 0, jumpVel.Z()}
	collisionBB = movement.BoundingBox()

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		xVel = game.BBClipCollide(blockBox, collisionBB, xVel, false, nil)
	}
	collisionBB = collisionBB.Translate(xVel)

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		zVel = game.BBClipCollide(blockBox, collisionBB, zVel, false, nil)
	}
	collisionBB = collisionBB.Translate(zVel)

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		yVel = game.BBClipCollide(blockBox, collisionBB, yVel, false, nil)
	}
	return yVel[1] != jumpVel[1] && xVel[0] == jumpVel[0] && zVel[2] == jumpVel[2]
}

func tryCollisions(p *player.Player, src world.BlockSource, dbg *player.Debugger, useSlideOffset bool, clientJumpPrevented bool) {
	var completedStep bool

	movement := p.Movement()
	collisionBB := movement.BoundingBox()
	currVel := movement.Vel()
	bbList := utils.GetNearbyBBoxes(collisionBB.Extend(currVel), src)
	//oneWayBlocks := utils.OneWayCollisionBlocks(utils.GetNearbyBlocks(collisionBB.Extend(currVel), false, false, w))

	// TODO: determine more blocks that are considered to be one-way physics blocks, and
	// figure out how to calculate ActorCollision::isStuckItem()
	useOneWayCollisions := movement.StuckInCollider()
	penetration := mgl32.Vec3{}

	yVel := mgl32.Vec3{0, currVel.Y()}
	if clientJumpPrevented {
		yVel[1] = 0
	}
	xVel := mgl32.Vec3{currVel.X()}
	zVel := mgl32.Vec3{0, 0, currVel.Z()}

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		yVel = game.BBClipCollide(blockBox, collisionBB, yVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(yVel)
	dbg.Notify(player.DebugModeMovementSim, true, "Y-collision non-step=%v /w penetration=%v (oneWay=%v)", yVel, penetration, useOneWayCollisions)

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		xVel = game.BBClipCollide(blockBox, collisionBB, xVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(xVel)
	dbg.Notify(player.DebugModeMovementSim, true, "(X) hz-collision non-step=%v /w penetration=%v (oneWay=%v)", xVel, penetration, useOneWayCollisions)

	for index := len(bbList) - 1; index >= 0; index-- {
		blockBox := bbList[index]
		zVel = game.BBClipCollide(blockBox, collisionBB, zVel, useOneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(zVel)
	dbg.Notify(player.DebugModeMovementSim, true, "(Z) hz-collision non-step=%v /w penetration=%v (oneWay=%v)", zVel, penetration, useOneWayCollisions)

	collisionVel := yVel.Add(xVel).Add(zVel)
	collisionPos := mgl32.Vec3{
		(collisionBB.Min().X() + collisionBB.Max().X()) * 0.5,
		collisionBB.Min().Y(),
		(collisionBB.Min().Z() + collisionBB.Max().Z()) * 0.5,
	}
	dbg.Notify(player.DebugModeMovementSim, true, "endCollisionVel=%v endCollisionPos=%v", collisionVel, collisionPos)

	hasPenetration := penetration.LenSqr() >= 9.999999999999999e-12
	movement.SetStuckInCollider(movement.PenetratedLastFrame() && hasPenetration)
	movement.SetPenetratedLastFrame(hasPenetration)

	xCollision := currVel.X() != collisionVel.X()
	yCollision := (currVel.Y() != collisionVel.Y()) || clientJumpPrevented
	zCollision := currVel.Z() != collisionVel.Z()
	onGround := movement.OnGround() || (yCollision && currVel.Y() < 0.0)

	if onGround && (xCollision || zCollision) {
		stepYVel := mgl32.Vec3{0, game.StepHeight}
		stepXVel := mgl32.Vec3{currVel.X()}
		stepZVel := mgl32.Vec3{0, 0, currVel.Z()}

		stepBB := movement.BoundingBox()
		for _, blockBox := range bbList {
			stepYVel = game.BBClipCollide(blockBox, stepBB, stepYVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(stepYVel)
		dbg.Notify(player.DebugModeMovementSim, true, "stepYVel=%v", stepYVel)

		for _, blockBox := range bbList {
			stepXVel = game.BBClipCollide(blockBox, stepBB, stepXVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(stepXVel)
		dbg.Notify(player.DebugModeMovementSim, true, "stepXVel=%v", stepXVel)
		for _, blockBox := range bbList {
			stepZVel = game.BBClipCollide(blockBox, stepBB, stepZVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(stepZVel)
		dbg.Notify(player.DebugModeMovementSim, true, "stepZVel=%v", stepZVel)

		inverseYStepVel := stepYVel.Mul(-1)
		for _, blockBox := range bbList {
			inverseYStepVel = game.BBClipCollide(blockBox, stepBB, inverseYStepVel, useOneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(inverseYStepVel)
		stepYVel = stepYVel.Add(inverseYStepVel)
		dbg.Notify(player.DebugModeMovementSim, true, "inverseYStepVel=%v", inverseYStepVel)

		stepVel := stepYVel.Add(stepXVel).Add(stepZVel)
		newBBList := utils.GetNearbyBBoxes(stepBB, src)
		stepPos := mgl32.Vec3{
			(stepBB.Min().X() + stepBB.Max().X()) * 0.5,
			stepBB.Min().Y(),
			(stepBB.Min().Z() + stepBB.Max().Z()) * 0.5,
		}
		dbg.Notify(player.DebugModeMovementSim, true, "endStepVel=%v endStepPos=%v", stepVel, stepPos)
		dbg.Notify(player.DebugModeMovementSim, true, "newBBList count: %d", len(newBBList))

		if len(newBBList) == 0 && game.Vec3HzDistSqr(collisionVel) < game.Vec3HzDistSqr(stepVel) {
			// HACK: If a step is possible here, we check which the client seems to align itself better with. The reason this is neccessary
			// is because in some scenarios, the client seems to reject a step even though Oomph thinks it is possible. This is mainly in scenarios
			// where the player teleports into a block.
			stepPosDist := stepPos.Sub(movement.Client().Pos()).Len()
			collisionPosDist := collisionPos.Sub(movement.Client().Pos()).Len()

			// We also need to ensure that the client isn't using this mechanic to create some weird movement bypass, so we will check if the
			// collisionPosDist is within the correction threshold. Even if the stepPosDist is greater than the correction threshold, Oomph is predicting
			// a step here anyway so it would make zero difference.
			if collisionPosDist > p.Opts().Movement.CorrectionThreshold || stepPosDist <= collisionPosDist {
				collisionVel = stepVel
				collisionBB = stepBB

				// This sliding offset is only used in versions 1.20.60 and below. On newer versions of the game, this sliding offset is no longer used.
				if useSlideOffset {
					completedStep = true
					slideOffset := movement.SlideOffset().Mul(game.SlideOffsetMultiplier)
					slideOffset[1] += stepVel.Y()
					movement.SetSlideOffset(slideOffset)
				}
				dbg.Notify(player.DebugModeMovementSim, true, "step successful")
			} else {
				dbg.Notify(player.DebugModeMovementSim, true, "step failed (client rejection) [clientPos=%v collisionPos=%v stepPos=%v]", movement.Client().Pos(), collisionPos, stepPos)
			}
		} else {
			dbg.Notify(player.DebugModeMovementSim, true, "step failed")
		}
	}

	// We use the bounding box instead of oldPos.Add(collisionVel) to calculate the final position of the player because
	// it is accurate to vanilla's logic. Furthermore, it is useful such as in cases where the slide offset is being used
	// by older versions to calculate collisions.
	endPos := mgl32.Vec3{
		(collisionBB.Min().X() + collisionBB.Max().X()) * 0.5,
		collisionBB.Min().Y(),
		(collisionBB.Min().Z() + collisionBB.Max().Z()) * 0.5,
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

	// Unlike Java, bedrock seems to have a strange condition for the client to be considered on the ground. This is probably useful
	// in cases where the client is teleporting, and the velocity (0) would still be equal to the previous velocity.
	movement.SetOnGround((yCollision && currVel.Y() < 0) || (movement.OnGround() && !yCollision && math32.Abs(currVel.Y()) <= 1e-5))
	checkSupportingBlockPos(movement, src, currVel)
	movement.SetVel(collisionVel)

	dbg.Notify(player.DebugModeMovementSim, true, "clientVel=%v clientPos=%v", movement.Client().Mov(), movement.Client().Pos())
	dbg.Notify(player.DebugModeMovementSim, true, "finalVel=%v finalPos=%v", collisionVel, movement.Pos())

	dbg.Notify(player.DebugModeMovementSim, true, "(client) hzCollision=%v yCollision=%v", movement.Client().HorizontalCollision(), movement.Client().VerticalCollision())
	dbg.Notify(player.DebugModeMovementSim, true, "(server) xCollision=%v yCollision=%v zCollision=%v", movement.XCollision(), movement.YCollision(), movement.ZCollision())
}

// avoidEdge is the function that helps the movement component remain at the edge of a block when sneaking.
func avoidEdge(movement player.MovementComponent, src world.BlockSource, dbg *player.Debugger) {
	if !movement.Sneaking() || !movement.OnGround() || movement.Vel().Y() > 0 {
		dbg.Notify(
			player.DebugModeMovementSim,
			true,
			"avoidEdge: conditions not met (sneaking=%v onGround=%v yVel=%v)",
			movement.Sneaking(),
			movement.OnGround(),
			movement.Vel().Y(),
		)
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

	for xMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, 0}), src)) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}
	}

	for zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -game.StepHeight * 1.01, zMov}), src)) == 0 {
		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	for xMov != 0.0 && zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, zMov}), src)) == 0 {
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

	oldVel := movement.Vel()
	newVel[0] = xMov
	newVel[2] = zMov
	movement.SetVel(newVel)

	dbg.Notify(player.DebugModeMovementSim, true, "(avoidEdge): oldVel=%v newVel=%v", oldVel, newVel)
}

func blocksInside(movement player.MovementComponent, src world.BlockSource) ([]world.Block, bool) {
	bb := movement.BoundingBox()
	blocks := []world.Block{}

	for _, result := range utils.GetNearbyBlocks(bb.Grow(1), false, true, src) {
		pos := result.Position
		block := result.Block
		boxes := utils.BlockCollisions(block, pos, src)

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

func attemptJump(p *player.Player, dbg *player.Debugger, clientJumpPrevented *bool) bool {
	movement := p.Movement()
	if !movement.Jumping() || !movement.OnGround() || movement.JumpDelay() > 0 {
		dbg.Notify(player.DebugModeMovementSim, movement.Jumping(), "rejected jump from client (onGround=%v jumpDelay=%d)", movement.OnGround(), movement.JumpDelay())
		return false
	}

	newVel := movement.Vel()
	newVel[1] = math32.Max(movement.JumpHeight(), newVel[1])
	movement.SetJumpDelay(game.JumpDelayTicks)

	if movement.Sprinting() {
		force := movement.Rotation().Z() * 0.017453292
		newVel[0] -= game.MCSin(force) * 0.2
		newVel[2] += game.MCCos(force) * 0.2
	}

	// TODO: There is probably a more efficient/proper way the bedrock client handles blocking it's own jump, but this
	// is functional for now.
	if clientJumpPrevented != nil && !movement.HasKnockback() && !movement.HasTeleport() && isJumpBlocked(p, newVel) {
		*clientJumpPrevented = true
		p.Dbg.Notify(player.DebugModeMovementSim, true, "jump determined to be blocked")
	}

	movement.SetVel(newVel)
	return true
}

func attemptTeleport(p *player.Player, dbg *player.Debugger) bool {
	movement := p.Movement()
	if !movement.HasTeleport() {
		return false
	}

	if !movement.TeleportSmoothed() {
		movement.SetPos(movement.TeleportPos())
		movement.SetVel(mgl32.Vec3{})
		movement.SetJumpDelay(0)
		attemptJump(p, dbg, nil)
		return true
	}
	// Calculate the smooth teleport's next position.
	posDelta := movement.TeleportPos().Sub(movement.Pos())
	if remaining := movement.RemainingTeleportTicks() + 1; remaining > 0 {
		newPos := movement.Pos().Add(posDelta.Mul(1.0 / float32(remaining)))
		movement.SetPos(newPos)
		//movement.SetVel(mgl32.Vec3{})
		movement.SetJumpDelay(0)
		return remaining > 1
	}
	return false
}

func findSupportingBlock(movement player.MovementComponent, bb cube.BBox, w world.BlockSource) {
	var (
		blockPos  *cube.Pos
		minDist   = float32(math.MaxFloat32 - 1)
		centerPos = cube.PosFromVec3(movement.Pos()).Vec3().Add(mgl32.Vec3{0.5, 0.5, 0.5})
	)
	for result := range utils.GetNearbyBlockCollisions(bb, w) {
		if dist := result.Position.Vec3().Sub(centerPos).LenSqr(); dist < minDist {
			minDist = dist
			blockPos = &result.Position
		}
	}
	movement.SetSupportingBlockPos(blockPos)
}

func checkSupportingBlockPos(movement player.MovementComponent, src world.BlockSource, vel mgl32.Vec3) {
	if !movement.OnGround() {
		movement.SetSupportingBlockPos(nil)
		return
	}
	decBB := movement.BoundingBox().ExtendTowards(cube.FaceDown, 1e-3) //.GrowVec3(mgl32.Vec3{0.025, 0, 0.025})
	findSupportingBlock(movement, decBB, src)
	if movement.SupportingBlockPos() == nil {
		decBB = decBB.Translate(mgl32.Vec3{-vel[0], 0, -vel[2]})
		findSupportingBlock(movement, decBB, src)
	}
}
