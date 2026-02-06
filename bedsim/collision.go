package bedsim

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

type clipCollideResult struct {
	depenetratingAxis     int
	penetration           float64
	clippedVelocity       mgl64.Vec3
	depenetratingVelocity mgl64.Vec3
}

// BBClipCollide clips or depenetrates a moving bounding box against a stationary one.
func BBClipCollide(this, c cube.BBox, vel mgl64.Vec3, oneWay bool, penetration *mgl64.Vec3) mgl64.Vec3 {
	result := doBBClipCollide(this, c, vel)
	if penetration != nil && penetration[result.depenetratingAxis] < result.penetration {
		penetration[result.depenetratingAxis] = result.penetration
	}

	if oneWay {
		return result.clippedVelocity
	}
	return result.depenetratingVelocity
}

func doBBClipCollide(stationary, moving cube.BBox, velocity mgl64.Vec3) (result clipCollideResult) {
	result.clippedVelocity = velocity
	result.depenetratingVelocity = velocity

	if BBHasZeroVolume(stationary) {
		return
	}

	axisPenetrations := [3]float64{}
	axisPenetrationsSigned := [3]float64{}
	normalDirs := [3]float64{}
	seperatingAxes, seperatingAxis := 0, 0
	resultPenetration := math.MaxFloat64 - 1

	for i := range 3 {
		minPenetration := moving.Max()[i] - stationary.Min()[i]
		maxPenetration := stationary.Max()[i] - moving.Min()[i]

		if math.Abs(minPenetration) <= 1e-7 {
			minPenetration = 0
		}
		if math.Abs(maxPenetration) <= 1e-7 {
			maxPenetration = 0
		}

		minPositive := math.Max(0, minPenetration)
		maxPositive := math.Max(0, maxPenetration)

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
		resultPenetration = math.Min(resultPenetration, axisPenetrations[i])
	}

	// No separating axes means a collision.
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
			result.depenetratingVelocity[bestAxis] = math.Max(desiredVelocity, velocity[bestAxis])
		} else {
			result.depenetratingVelocity[bestAxis] = math.Min(desiredVelocity, velocity[bestAxis])
		}
		result.depenetratingAxis = bestAxis
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

// BBHasZeroVolume returns true if the bounding box has zero volume.
func BBHasZeroVolume(bb cube.BBox) bool {
	return bb.Min() == bb.Max()
}
