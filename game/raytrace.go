package game

import (
	"iter"

	"github.com/chewxy/math32"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// https://github.com/pmmp/Math/blob/stable/src/VoxelRayTrace.php#L67
func BlocksBetween(start, end mgl32.Vec3) iter.Seq[mgl32.Vec3] {
	return func(yield func(mgl32.Vec3) bool) {
		dirVec := end.Sub(start).Normalize()
		if dirVec.LenSqr() <= 0 {
			return
		}

		radius := start.Sub(end).Len()
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

		currentBlock := cube.PosFromVec3(start).Vec3()
		for {
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
