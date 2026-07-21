package game

import (
	"iter"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// BlockPosFromVec3 returns the block position containing a float32 vector.
func BlockPosFromVec3(vec mgl32.Vec3) cube.Pos {
	return cube.Pos{int(math32.Floor(vec[0])), int(math32.Floor(vec[1])), int(math32.Floor(vec[2]))}
}

// BlockPosVec3 returns the float32 vector representation of a block position.
func BlockPosVec3(pos cube.Pos) mgl32.Vec3 {
	return mgl32.Vec3{float32(pos[0]), float32(pos[1]), float32(pos[2])}
}

// https://github.com/pmmp/Math/blob/stable/src/VoxelRayTrace.php#L67
func BlocksBetween(start, end mgl32.Vec3, maxSafeIters int) iter.Seq[mgl32.Vec3] {
	currIters := 0
	return func(yield func(mgl32.Vec3) bool) {
		delta := end.Sub(start)
		if delta.LenSqr() <= 1e-10 {
			return
		}
		dirVec := delta.Normalize()
		radius := delta.Len()
		stepX := PHPSpaceshipOp(dirVec.X(), 0)
		stepY := PHPSpaceshipOp(dirVec.Y(), 0)
		stepZ := PHPSpaceshipOp(dirVec.Z(), 0)

		tMaxX := rayTraceDistanceToBoundary(start.X(), dirVec.X())
		tMaxY := rayTraceDistanceToBoundary(start.Y(), dirVec.Y())
		tMaxZ := rayTraceDistanceToBoundary(start.Z(), dirVec.Z())

		tDeltaX := float32(0)
		if dirVec.X() != 0 {
			tDeltaX = stepX / dirVec.X()
		}

		tDeltaY := float32(0)
		if dirVec.Y() != 0 {
			tDeltaY = stepY / dirVec.Y()
		}

		tDeltaZ := float32(0)
		if dirVec.Z() != 0 {
			tDeltaZ = stepZ / dirVec.Z()
		}

		currentBlock := BlockPosVec3(BlockPosFromVec3(start))
		for {
			currIters++
			if currIters > maxSafeIters {
				return
			}

			if !yield(currentBlock) {
				return
			}

			if tMaxX < tMaxY && tMaxX < tMaxZ {
				if tMaxX > radius {
					return
				}
				currentBlock = currentBlock.Add(mgl32.Vec3{stepX})
				tMaxX += tDeltaX
			} else if tMaxY < tMaxZ {
				if tMaxY > radius {
					return
				}
				currentBlock = currentBlock.Add(mgl32.Vec3{0, stepY})
				tMaxY += tDeltaY
			} else {
				if tMaxZ > radius {
					return
				}
				currentBlock = currentBlock.Add(mgl32.Vec3{0, 0, stepZ})
				tMaxZ += tDeltaZ
			}
		}
	}
}

// https://github.com/pmmp/Math/blob/stable/src/VoxelRayTrace.php#L134
func rayTraceDistanceToBoundary(s, ds float32) float32 {
	if ds == 0 {
		return math32.MaxFloat32
	}

	if ds < 0 {
		s = -s
		ds = -ds

		if math32.Floor(s) == s {
			return 0
		}
	}

	return (1 - (s - math32.Floor(s))) / ds
}
