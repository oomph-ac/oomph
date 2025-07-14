package simulation

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
)

func updateInWaterStateAndDoWaterCurrentPushing(p *player.Player) {
	// TODO: Account for vehicle logic in liquids.
	p.Movement().SetInWater(updateFluidHeightAndDoFluidPushing[block.Water](p, 0.014))
}

func liquidFlow(p *player.Player, blockPos df_cube.Pos, liquidBlock world.Liquid) mgl32.Vec3 {
	//const maxFluidLevel float32 = 8.0 / 9.0
	fluidLevel := utils.FluidLevelAt(p.World(), blockPos)

	var d0, d1 float32
	for _, face := range df_cube.HorizontalFaces() {
		facePos := blockPos.Side(face)
		if !affectsFlow(p.World(), blockPos, facePos) {
			continue
		}
		var f, f1 float32 = utils.FluidLevelAt(p.World(), facePos), 0.0
		if f == 0.0 {
			mat := p.World().Block(facePos)
			matBBs := utils.BlockBoxes(mat, facePos, p.World())
			altPos := df_cube.Pos{blockPos[0], blockPos[1] - 1, blockPos[2]}
			if len(matBBs) > 0 && affectsFlow(p.World(), blockPos, altPos) {
				f = utils.FluidLevelAt(p.World(), altPos)
				if f > 0 {
					f1 = fluidLevel - f // MCP: (fluidLevel - (f - 8/9))
				}
			}
		} else if f > 0 { // ?????
			f1 = fluidLevel - f
		}

		if f1 != 0.0 {
			d0 += float32(facePos.X()) * f1
			d1 += float32(facePos.Z()) * f1
		}
	}

	flowVel := mgl32.Vec3{d0, 0, d1}
	if liquidBlock.LiquidFalling() {
		for _, face := range df_cube.HorizontalFaces() {
			facePos := blockPos.Side(face)
			altFacePos := facePos
			altFacePos[1]++

			blockFirst, blockSecond := p.World().Block(facePos), p.World().Block(altFacePos)
			solidFirst, solidSecond := len(utils.BlockBoxes(blockFirst, facePos, p.World())) > 0, len(utils.BlockBoxes(blockSecond, altFacePos, p.World())) > 0
			if solidFirst || solidSecond {
				flowVel = utils.NormalizeVec32(flowVel).Add(mgl32.Vec3{0, -6.0, 0})
			}
		}
	}
	return utils.NormalizeVec32(flowVel)
}

func affectsFlow(src world.BlockSource, originPos, newPos df_cube.Pos) bool {
	l1, isLiquid1 := src.Block(originPos).(world.Liquid)
	l2, isLiquid2 := src.Block(newPos).(world.Liquid)
	return !isLiquid1 || (isLiquid1 && isLiquid2 && l1.LiquidType() == l2.LiquidType())
}

func updateFluidHeightAndDoFluidPushing[T world.Liquid](p *player.Player, someScale float32) bool {
	movement := p.Movement()
	deflatedBB := movement.BoundingBox().Grow(-0.001)

	fluidHeight, hasTouched := float32(0), false
	vel := mgl32.Vec3{}
	k1 := 0

	for _, blockResult := range utils.GetNearbyBlocks(deflatedBB, false, false, p.World()) {
		liquid, isTargetLiquid := blockResult.Block.(T)
		if !isTargetLiquid {
			continue
		}
		blockPos := blockResult.Position
		d1 := float32(blockPos.Y()) + utils.FluidLevelAt(p.World(), blockPos)
		if d1 < deflatedBB.Min().Y() {
			continue
		}
		hasTouched = true
		fluidHeight = math32.Max(fluidHeight, d1-deflatedBB.Min().Y())
		flow := liquidFlow(p, df_cube.Pos(blockPos), liquid)
		if fluidHeight < 0.4 {
			flow = flow.Mul(fluidHeight)
		}
		vel = vel.Add(flow)
	}

	if vel.LenSqr() > 0 {
		if k1 > 0 {
			vel = vel.Mul(1.0 / float32(k1))
		}
		playerVel := p.Movement().Vel()
		vel = vel.Mul(someScale)
		const d2, d3 float32 = 0.003, 0.0045
		if math32.Abs(playerVel.X()) < d2 && math32.Abs(playerVel.Y()) < d2 && math32.Abs(playerVel.Z()) < d2 && vel.Len() < d3 {
			vel = utils.NormalizeVec32(vel).Mul(d3)
		}
		movement.SetVel(vel)
	}

	movement.SetFluidHeight(fluidHeight)
	return hasTouched
}
