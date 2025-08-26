package movement

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
)

func (ctx *movementContext) updateInWaterStateAndDoFluidPushing() {
	movement := ctx.mPlayer.Movement()
	movement.SetWaterHeight(0.0)
	movement.SetLavaHeight(0.0)
	ctx.updateInWaterStateAndDoWaterCurrentPushing()
	// TODO: account for lava speed in nether
	d0 := float32(0.0023333333333333335)
	_ = ctx.updateFluidHeightAndDoFluidPushing(d0, false)
	// return movement.WasTouchingWater() || flag
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "updateInWaterStateAndDoFluidPushing: waterHeight=%.3f lavaHeight=%.3f wasTouchingWater=%t", movement.WaterHeight(), movement.LavaHeight(), movement.WasTouchingWater())
}

func (ctx *movementContext) updateInWaterStateAndDoWaterCurrentPushing() {
	flag := ctx.updateFluidHeightAndDoFluidPushing(0.014, true)
	ctx.mPlayer.Movement().SetWasTouchingWater(flag)
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "updateInWaterStateAndDoWaterCurrentPushing: flag=%t waterHeight=%.3f", flag, ctx.mPlayer.Movement().WaterHeight())
}

func (ctx *movementContext) updateFluidOnEyes() {
	movement := ctx.mPlayer.Movement()
	movement.SetWaterOnEyes(false)
	movement.SetLavaOnEyes(false)
	eyePos := movement.Pos()
	eyePos[1] += movement.EyeHeight()
	blockPos := [3]int(cube.PosFromVec3(eyePos))
	liquid, ok := ctx.mPlayer.World().Block(blockPos).(world.Liquid)
	if !ok {
		return
	}
	d1 := float32(blockPos[1]) + utils.LiquidHeight(liquid, blockPos, ctx.mPlayer.World())
	if d1 >= eyePos[1] {
		if _, isWater := liquid.(block.Water); isWater {
			movement.SetWaterOnEyes(true)
			ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[liquid] updateFluidOnEyes waterOnEyes=true d=%.3f eyeY=%.3f", d1, eyePos[1])
		} else {
			movement.SetLavaOnEyes(true)
			ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[liquid] updateFluidOnEyes lavaOnEyes=true d=%.3f eyeY=%.3f", d1, eyePos[1])
		}
	}
}

func (ctx *movementContext) applyFluidFallingAdjustedMovement() {
	movement := ctx.mPlayer.Movement()
	vel := movement.Vel()
	flag := movement.Vel().Y() <= 0.0
	gravity := movement.Gravity()
	if gravity != 0.0 && !movement.Sprinting() {
		const mul float32 = 1 / 16.0
		var d0 float32
		prevY := vel[1]
		if flag && math32.Abs(vel[1]-0.005) >= 0.003 && math32.Abs(vel[1]-gravity*mul) < 0.003 {
			d0 = -0.003
		} else {
			d0 = vel[1] - gravity*mul
		}
		vel[1] = d0
		movement.SetVel(vel)
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[liquid] applyFluidFallingAdjustedMovement gravity=%.3f prevY=%.3f newY=%.3f", gravity, prevY, d0)
	}
}

func (ctx *movementContext) updateFluidHeightAndDoFluidPushing(scale float32, isWater bool) bool {
	movement := ctx.mPlayer.Movement()
	bb := movement.BoundingBox().Grow(-0.001)
	d0 := float32(0.0)
	flag1 := false
	newVel := mgl32.Vec3{}
	k1 := 0
	for _, blockResult := range utils.GetNearbyBlocks(bb, false, false, ctx.mPlayer.World()) {
		var (
			l  world.Liquid
			ok bool
		)
		if isWater {
			l, ok = blockResult.Block.(block.Water)
		} else {
			l, ok = blockResult.Block.(block.Lava)
		}
		if !ok {
			continue
		}
		d1 := float32(blockResult.Position[1]) + utils.LiquidHeight(l, blockResult.Position, ctx.mPlayer.World())
		if d1 >= bb.Min().Y() {
			flag1 = true
			d0 = math32.Max(d0, d1-bb.Min().Y())
			liquidFlow := utils.LiquidFlow(l, blockResult.Position, ctx.mPlayer.World())
			if d0 < 0.4 {
				liquidFlow = liquidFlow.Mul(d0)
			}
			newVel = newVel.Add(liquidFlow)
			k1++
		}
	}

	if newVel.LenSqr() > 0 {
		if k1 > 0 {
			newVel = newVel.Mul(1.0 / float32(k1))
		}
		avgFlow := newVel
		currVel := movement.Vel()
		newVel = newVel.Mul(scale)
		const d2, d3 float32 = 0.003, 0.0045000000000000005
		if math32.Abs(currVel[0]) < d2 && math32.Abs(currVel[2]) < d2 && currVel.Len() < d3 {
			newVel = newVel.Normalize().Mul(d3)
		}
		finalVel := currVel.Add(newVel)
		movement.SetVel(finalVel)
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[liquid] updateFluidHeightAndDoFluidPushing isWater=%t blocks=%d height=%.3f avgFlow=%v addedVel=%v resultVel=%v", isWater, k1, d0, avgFlow, newVel, finalVel)
	}

	if isWater {
		movement.SetWaterHeight(d0)
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[liquid] waterHeight=%.3f flag=%t", movement.WaterHeight(), flag1)
	} else {
		movement.SetLavaHeight(d0)
		ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[liquid] lavaHeight=%.3f flag=%t", movement.LavaHeight(), flag1)
	}
	return flag1
}

func (ctx *movementContext) goUpInWater() {
	vel := ctx.mPlayer.Movement().Vel()
	vel[1] += 0.04
	ctx.mPlayer.Movement().SetVel(vel)
}

func (ctx *movementContext) goDownInWater() {
	vel := ctx.mPlayer.Movement().Vel()
	vel[1] -= 0.04
	ctx.mPlayer.Movement().SetVel(vel)
}
