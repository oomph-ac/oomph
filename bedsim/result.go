package bedsim

import "github.com/go-gl/mathgl/mgl64"

// SimulationOutcome describes which path the simulator took for the current tick.
type SimulationOutcome uint8

const (
	SimulationOutcomeNormal SimulationOutcome = iota
	SimulationOutcomeTeleport
	SimulationOutcomeUnreliable
	SimulationOutcomeUnloadedChunk
	SimulationOutcomeImmobileOrNotReady
)

// SimulationResult captures the outcome of a single simulation tick.
type SimulationResult struct {
	Position mgl64.Vec3
	Velocity mgl64.Vec3
	Movement mgl64.Vec3

	OnGround bool
	CollideX bool
	CollideY bool
	CollideZ bool

	PositionDelta   mgl64.Vec3
	VelocityDelta   mgl64.Vec3
	NeedsCorrection bool

	Outcome SimulationOutcome
}
