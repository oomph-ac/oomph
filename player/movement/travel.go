package movement

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (ctx *movementContext) travel() {
	movement := ctx.mPlayer.Movement()
	inWater, inLava := movement.WasTouchingWater(), movement.LavaHeight() > 0.0
	if inWater || inLava {
		ctx.travelFluid()
	} else {
		if ctx.attemptGlide() {
			return
		}
		ctx.travelInAir()
	}
}

func (ctx *movementContext) travelInAir() {
	ctx.searchBlockUnder()
	ctx.updateFrictionAndSpeed()
	ctx.applyKnockback()
	ctx.jump()
	ctx.moveRelative()
	ctx.climb()
	ctx.applyPreBlockSlowdown()
	ctx.avoidEdge()
	ctx.tryCollisions()
	ctx.walkOnBlock()
	ctx.searchBlockUnder()
	ctx.applyPostCollisions()
	ctx.applyPostBlockSlowdown()
	movement := ctx.mPlayer.Movement()
	newVel := movement.Vel()
	if eff, ok := ctx.mPlayer.Effects().Get(packet.EffectLevitation); ok {
		levSpeed := game.LevitationGravityMultiplier * float32(eff.Amplifier)
		newVel[1] += (levSpeed - newVel[1]) * 0.2
	} else {
		newVel[1] -= movement.Gravity()
		newVel[1] *= game.NormalGravityMultiplier
	}
	newVel[0] *= ctx.blockFriction
	newVel[2] *= ctx.blockFriction
	movement.SetVel(newVel)

	sPos, cPos := movement.Pos(), movement.Client().Pos()
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelNormal] endOfFrameVel=%v", newVel)
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelNormal] serverPos=%v clientPos=%v, diff=%v", sPos, cPos, sPos.Sub(cPos))
}

func (ctx *movementContext) travelFluid() {
	movement := ctx.mPlayer.Movement()
	if movement.PressingSneak() {
		ctx.goDownInWater()
	}
	if movement.PressingJump() {
		ctx.goUpInWater()
	}

	d0 := movement.Pos().Y()
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] start inWater=%t waterHeight=%.3f lavaHeight=%.3f posY=%.3f vel=%v", movement.WasTouchingWater(), movement.WaterHeight(), movement.LavaHeight(), movement.Pos().Y(), movement.Vel())
	if movement.WasTouchingWater() {
		f := float32(0.8)
		if movement.Sprinting() {
			f = 0.9
		}
		f1, f2 := float32(0.02), movement.WaterSpeed()
		if !movement.OnGround() {
			f2 *= 0.5
		}
		if f2 > 0 {
			f += (0.546 - f) * f2
			f1 += (movement.MovementSpeed() - f1) * f2
		}
		// TODO: Check for dolphin grace effect.
		ctx.moveRelativeSpeed = f1
		ctx.moveRelative()
		ctx.tryCollisions()
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] water pre-apply f=%.3f f1=%.3f f2=%.3f vel=%v", f, f1, f2, movement.Vel())
		// TODO: check if we actually need to simulate climbs here like in java.
		//ctx.climb()
		vel := movement.Vel()
		vel[0] *= f
		vel[1] *= 0.8
		vel[2] *= f
		movement.SetVel(vel)
		ctx.applyFluidFallingAdjustedMovement()
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] water post-apply vel=%v", movement.Vel())
	} else {
		ctx.moveRelativeSpeed = 0.02
		ctx.moveRelative()
		ctx.tryCollisions()
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] lava pre-apply lavaHeight=%.3f vel=%v", movement.LavaHeight(), movement.Vel())
		// TODO: properly account for eye heights at different player models (sneaking, swimming, etc.)
		fluidJumpThreshold := float32(0.4)
		if movement.LavaHeight() <= fluidJumpThreshold {
			vel := movement.Vel()
			vel[0] *= 0.5
			vel[1] *= 0.8
			vel[2] *= 0.5
			movement.SetVel(vel)
			ctx.applyFluidFallingAdjustedMovement()
			ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] lava post-apply shallow vel=%v", movement.Vel())
		} else {
			movement.SetVel(movement.Vel().Mul(0.5))
			ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] lava post-apply deep vel=%v", movement.Vel())
		}

		if gravity := movement.Gravity(); gravity != 0.0 {
			vel := movement.Vel()
			vel[1] -= gravity * 0.25
			movement.SetVel(vel)
		}
	}

	freeVel := movement.Vel()
	freeVel[1] += 0.6 - movement.Pos().Y() + d0
	if (movement.XCollision() || movement.ZCollision()) && ctx.isFree(freeVel) {
		vel := movement.Vel()
		vel[1] *= 0.3
		movement.SetVel(vel)
	}
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] vel=%v pos=%v", movement.Vel(), movement.Pos())
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] clientVel=%v clientPos=%v", movement.Client().Vel(), movement.Client().Pos())
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelFluid] velDiff=%v posDiff=%v", movement.Vel().Sub(movement.Client().Vel()), movement.Pos().Sub(movement.Client().Pos()))
}
