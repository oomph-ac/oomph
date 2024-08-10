package utils

import (
	"github.com/chewxy/math32"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

type clipCollideResult struct {
	depenetratingAxis     int
	penetration           float32
	clippedVelocity       mgl32.Vec3
	depenetratingVelocity mgl32.Vec3
}

func BBClipCollide(this, c cube.BBox, vel mgl32.Vec3, oneWay bool, penetration *float32) mgl32.Vec3 {
	result := doBBClipCollide(this, c, vel)
	if penetration != nil {
		*penetration = result.penetration
	}

	if oneWay {
		return result.clippedVelocity
	}
	return result.depenetratingVelocity
}

func doBBClipCollide(stationary, moving cube.BBox, velocity mgl32.Vec3) (result clipCollideResult) {
	result.clippedVelocity = velocity
	result.depenetratingVelocity = velocity

	if BBHasZeroVolume(stationary) {
		return
	}

	axisPenetrations := [3]float32{}
	axisPenetrationsSigned := [3]float32{}
	normalDirs := [3]float32{}
	seperatingAxes, seperatingAxis := 0, 0
	resultPenetration := float32(math32.MaxFloat32 - 1)

	for i := 0; i < 3; i++ {
		minPenetration := moving.Max()[i] - stationary.Min()[i]
		maxPenetration := stationary.Max()[i] - moving.Min()[i]

		if math32.Abs(minPenetration) <= 1e-7 {
			minPenetration = 0
		}
		if math32.Abs(maxPenetration) <= 1e-7 {
			maxPenetration = 0
		}

		minPositive := math32.Max(0, minPenetration)
		maxPositive := math32.Max(0, maxPenetration)

		if minPositive == 0 {
			axisPenetrations[i] = 0
			axisPenetrationsSigned[i] = minPenetration
			normalDirs[i] = -1
			seperatingAxes++
			seperatingAxis = i
		} else if maxPositive == 0 {
			axisPenetrations[i] = 0
			axisPenetrationsSigned[i] = maxPenetration
			normalDirs[i] = 1
			seperatingAxes++
			seperatingAxis = i
		} else if minPositive < maxPositive {
			axisPenetrations[i] = minPositive
			axisPenetrationsSigned[i] = minPositive
			normalDirs[i] = -1
		} else {
			axisPenetrations[i] = maxPositive
			axisPenetrationsSigned[i] = maxPositive
			normalDirs[i] = 1
		}

		if seperatingAxes > 1 {
			return
		}
		resultPenetration = math32.Min(resultPenetration, axisPenetrations[i])
	}

	// No separating axes means a collision
	if seperatingAxes == 0 {
		result.penetration = resultPenetration
		bestAxis := 0
		for i := 1; i < 3; i++ {
			if axisPenetrations[i] < axisPenetrations[bestAxis] {
				bestAxis = i
			}
		}

		desiredVelocity := axisPenetrations[bestAxis] * normalDirs[bestAxis]
		if desiredVelocity > 0 {
			result.depenetratingVelocity[bestAxis] = math32.Max(desiredVelocity, velocity[bestAxis])
		} else {
			result.depenetratingVelocity[bestAxis] = math32.Min(desiredVelocity, velocity[bestAxis])
		}
		return
	}

	sweptPenetration := axisPenetrationsSigned[seperatingAxis] - (normalDirs[seperatingAxis] * velocity[seperatingAxis])
	if sweptPenetration <= 0 {
		return
	}

	resolvedVelocity := axisPenetrationsSigned[seperatingAxis] * normalDirs[seperatingAxis]
	result.clippedVelocity[seperatingAxis] = resolvedVelocity
	result.depenetratingVelocity[seperatingAxis] = resolvedVelocity
	return
}

func BBHasZeroVolume(bb cube.BBox) bool {
	return bb.Min() == bb.Max()
}
