package movement

import (
	"github.com/chewxy/math32"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
)

func (ctx *movementContext) tryCollisions() {
	movement := ctx.mPlayer.Movement()
	blockSrc := ctx.mPlayer.World()
	dbg := ctx.mPlayer.Dbg

	var completedStep bool
	ctx.preCollideVel = movement.Vel()
	ctx.preCollideGround = movement.OnGround()

	collisionBB := movement.BoundingBox()
	currVel := movement.Vel()
	bbList := utils.GetNearbyBBoxes(collisionBB.Extend(currVel), blockSrc)

	// TODO: determine more blocks that are considered to be one-way physics blocks, and
	// figure out how to calculate ActorCollision::isStuckItem()
	useOneWayCollisions := movement.StuckInCollider()
	penetration := mgl32.Vec3{}

	yVel := mgl32.Vec3{0, currVel.Y()}
	if ctx.clientJumpPrevented {
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
	yCollision := (currVel.Y() != collisionVel.Y()) || ctx.clientJumpPrevented
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
		newBBList := utils.GetNearbyBBoxes(stepBB, blockSrc)
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
			if collisionPosDist > ctx.mPlayer.Opts().Movement.CorrectionThreshold || stepPosDist <= collisionPosDist {
				collisionVel = stepVel
				collisionBB = stepBB

				// This sliding offset is only used in versions 1.20.60 and below. On newer versions of the game, this sliding offset is no longer used.
				if ctx.useSlideOffset {
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
	if ctx.useSlideOffset {
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
	movement.SetVel(collisionVel)

	dbg.Notify(player.DebugModeMovementSim, true, "clientVel=%v clientPos=%v", movement.Client().Mov(), movement.Client().Pos())
	dbg.Notify(player.DebugModeMovementSim, true, "finalVel=%v finalPos=%v", collisionVel, movement.Pos())

	dbg.Notify(player.DebugModeMovementSim, true, "(client) hzCollision=%v yCollision=%v", movement.Client().HorizontalCollision(), movement.Client().VerticalCollision())
	dbg.Notify(player.DebugModeMovementSim, true, "(server) xCollision=%v yCollision=%v zCollision=%v", movement.XCollision(), movement.YCollision(), movement.ZCollision())
}

func (ctx *movementContext) walkOnBlock() {
	movement := ctx.mPlayer.Movement()
	if !movement.OnGround() || movement.Sneaking() {
		movement.SetMov(movement.Vel())
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "serverMov=%v", movement.Vel())
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "clientMov=%v", movement.Client().Mov())
		return
	}

	newVel := movement.Vel()
	switch utils.BlockName(ctx.blockUnder) {
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
	movement.SetMov(newVel)

	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "serverMov=%v", newVel)
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "clientMov=%v", movement.Client().Mov())
}

func (ctx *movementContext) applyPostCollisions() {
	movement := ctx.mPlayer.Movement()
	if !ctx.preCollideGround && movement.YCollision() {
		ctx.landOnBlock()
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

func (ctx *movementContext) landOnBlock() {
	movement := ctx.mPlayer.Movement()
	newVel := movement.Vel()
	if ctx.preCollideVel.Y() >= 0 || movement.PressingSneak() {
		newVel[1] = 0
		movement.SetVel(newVel)
		return
	}

	switch utils.BlockName(ctx.blockUnder) {
	case "minecraft:slime":
		newVel[1] = game.SlimeBounceMultiplier * ctx.preCollideVel.Y()
	case "minecraft:bed":
		newVel[1] = math32.Min(1.0, game.BedBounceMultiplier*ctx.preCollideVel.Y())
	default:
		newVel[1] = 0
	}
	movement.SetVel(newVel)
}

func (ctx *movementContext) isFree(vel mgl32.Vec3) bool {
	bb := ctx.mPlayer.Movement().BoundingBox().Translate(vel)
	return len(utils.GetNearbyBBoxes(bb, ctx.mPlayer.World())) == 0
}
