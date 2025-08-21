package movement

import (
	"github.com/chewxy/math32"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
)

type movementContext struct {
	mPlayer *player.Player

	preCollideVel    mgl32.Vec3
	preCollideGround bool

	blockUnder        world.Block
	blockFriction     float32
	moveRelativeSpeed float32

	hasBlockSlowdown    bool
	useSlideOffset      bool
	clientJumpPrevented bool
}

func (ctx *movementContext) tryTeleport() bool {
	movement := ctx.mPlayer.Movement()
	if !movement.HasTeleport() {
		return false
	}

	if !movement.TeleportSmoothed() {
		movement.SetPos(movement.TeleportPos())
		movement.SetVel(mgl32.Vec3{})
		movement.SetJumpDelay(0)
		ctx.jump()
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "teleported to %v", movement.Pos())
		return true
	}

	// Calculate the smooth teleport's next position.
	posDelta := movement.TeleportPos().Sub(movement.Pos())
	if remaining := movement.RemainingTeleportTicks() + 1; remaining > 0 {
		newPos := movement.Pos().Add(posDelta.Mul(1.0 / float32(remaining)))
		movement.SetPos(newPos)
		//movement.SetVel(mgl32.Vec3{})
		movement.SetJumpDelay(0)
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "smooth teleported to %v", movement.Pos())
		return remaining > 1
	}
	return false
}

func (ctx *movementContext) searchBlockUnder() {
	movement := ctx.mPlayer.Movement()
	ctx.blockUnder = ctx.mPlayer.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.5}))))
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "searchBlockUnder: blockUnder=%s", utils.BlockName(ctx.blockUnder))
}

func (ctx *movementContext) updateFrictionAndSpeed() {
	ctx.blockFriction = game.DefaultAirFriction
	ctx.moveRelativeSpeed = ctx.mPlayer.Movement().AirSpeed()
	if ctx.mPlayer.Movement().OnGround() {
		ctx.blockFriction *= utils.BlockFriction(ctx.blockUnder)
		ctx.moveRelativeSpeed = ctx.mPlayer.Movement().MovementSpeed() * (0.16277136 / (ctx.blockFriction * ctx.blockFriction * ctx.blockFriction))
	}
}

func (ctx *movementContext) applyKnockback() {
	movement := ctx.mPlayer.Movement()
	if movement.HasKnockback() {
		movement.SetVel(movement.Knockback())
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "applied knockback: %v", movement.Vel())
	}
}

func (ctx *movementContext) jump() {
	movement := ctx.mPlayer.Movement()
	if !movement.Jumping() || !movement.OnGround() || movement.JumpDelay() > 0 {
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, movement.Jumping(), "rejected jump from client (onGround=%v jumpDelay=%d)", movement.OnGround(), movement.JumpDelay())
		return
	}

	newVel := movement.Vel()
	newVel[1] = math32.Max(movement.JumpHeight(), newVel[1])
	movement.SetJumpDelay(game.JumpDelayTicks)
	if movement.Sprinting() {
		force := movement.Rotation().Z() * 0.017453292
		newVel[0] -= game.MCSin(force) * 0.2
		newVel[2] += game.MCCos(force) * 0.2
	}

	// FIXME: The client seems to sometimes prevent it's own jump from happening - it is unclear how it is determined, however.
	// This is a temporary hack to get around this issue for now.
	clientJump := movement.Client().Pos().Y() - movement.Client().LastPos().Y()
	hasBlockAbove := false
	jumpBB := movement.BoundingBox().Translate(newVel)
	for _, bb := range utils.GetNearbyBBoxes(jumpBB, ctx.mPlayer.World()) {
		if bb.Min().Y() > jumpBB.Min().Y() {
			hasBlockAbove = true
			break
		}
	}
	if hasBlockAbove && math32.Abs(clientJump) <= 1e-4 && !movement.HasKnockback() && !movement.HasTeleport() {
		ctx.clientJumpPrevented = true
	}
	movement.SetVel(newVel)
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "applied jump velocity (sprint=%t): %v", movement.Sprinting(), newVel)
}

func (ctx *movementContext) moveRelative() {
	movement := ctx.mPlayer.Movement()
	impulse := movement.Impulse()
	force := impulse.Y()*impulse.Y() + impulse.X()*impulse.X()

	if force >= 1e-4 {
		force = ctx.moveRelativeSpeed / math32.Max(math32.Sqrt(force), 1.0)
		mf, ms := impulse.Y()*force, impulse.X()*force

		yaw := movement.Rotation().Z() * math32.Pi / 180.0
		v2, v3 := game.MCSin(yaw), game.MCCos(yaw)

		newVel := movement.Vel()
		newVel[0] += ms*v3 - mf*v2
		newVel[2] += mf*v3 + ms*v2
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "applied moveRelative force [new=%v old=%v]", newVel, movement.Vel())
		movement.SetVel(newVel)
	}
}

func (ctx *movementContext) climb() {
	movement := ctx.mPlayer.Movement()
	nearClimbable := utils.BlockClimbable(ctx.mPlayer.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()))))
	if nearClimbable {
		newVel := movement.Vel()
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

		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "added climb velocity: %v (collided=%v pressingJump=%v)", newVel, movement.XCollision() || movement.ZCollision(), movement.PressingJump())
		movement.SetVel(newVel)
	}
}

func (ctx *movementContext) applyPreBlockSlowdown() {
	movement := ctx.mPlayer.Movement()
	blocksInside, isInsideBlock := ctx.blocksInside()
	stuckMultiplier, foundStuck := mgl32.Vec3{1, 1, 1}, false
	if !isInsideBlock {
		return
	}

	for _, b := range blocksInside {
		bName := utils.BlockName(b)
		if bName == "minecraft:sweet_berry_bush" {
			stuckMultiplier[0], stuckMultiplier[1], stuckMultiplier[2] = 0.8, 0.75, 0.8
			foundStuck = true
			break
		} else if bName == "minecraft:web" {
			stuckMultiplier[0], stuckMultiplier[1], stuckMultiplier[2] = 0.25, 0.05, 0.25
			foundStuck = true
			break
		}
	}
	if foundStuck {
		ctx.hasBlockSlowdown = true
		vel := movement.Vel()
		vel[0] *= stuckMultiplier[0]
		vel[1] *= stuckMultiplier[1]
		vel[2] *= stuckMultiplier[2]
		movement.SetVel(vel)
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "applied block slowdown: %v", vel)
	}
}

func (ctx *movementContext) applyPostBlockSlowdown() {
	if ctx.hasBlockSlowdown {
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "applied post-block slowdown")
		ctx.mPlayer.Movement().SetVel(mgl32.Vec3{})
	}
}

func (ctx *movementContext) blocksInside() ([]world.Block, bool) {
	movement := ctx.mPlayer.Movement()
	src := ctx.mPlayer.World()
	playerBB := movement.BoundingBox()
	blocks := []world.Block{}

	for _, result := range utils.GetNearbyBlocks(playerBB.Grow(1), false, true, src) {
		pos := result.Position
		block := result.Block
		boxes := utils.BlockCollisions(block, pos, src)

		for _, box := range boxes {
			if playerBB.IntersectsWith(box.Translate(pos.Vec3())) {
				blocks = append(blocks, block)
			}
		}
	}
	return blocks, len(blocks) > 0
}

func (ctx *movementContext) avoidEdge() {
	movement := ctx.mPlayer.Movement()
	src := ctx.mPlayer.World()
	dbg := ctx.mPlayer.Dbg

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
