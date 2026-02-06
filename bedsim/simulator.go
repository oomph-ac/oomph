package bedsim

// SimulationMode defines how strict the simulator should be with client corrections.
type SimulationMode uint8

const (
	SimulationModeAuthoritative SimulationMode = iota
	SimulationModePermissive
	SimulationModePassive
)

// SprintTiming defines when movement speed changes apply relative to simulation.
type SprintTiming uint8

const (
	SprintTimingModern SprintTiming = iota
	SprintTimingLegacy
)

// SimulationOptions define simulator behavior and correction thresholds.
type SimulationOptions struct {
	Mode SimulationMode

	PositionCorrectionThreshold float64
	VelocityCorrectionThreshold float64

	UseSlideOffset bool
	SprintTiming   SprintTiming

	LimitAllVelocity          bool
	LimitAllVelocityThreshold float64

	// Debugf receives internal simulation trace logs for callers that need deep diagnostics.
	Debugf func(format string, args ...any)
}

// Simulator orchestrates movement simulation using the provided adapters.
type Simulator struct {
	World     WorldProvider
	Effects   EffectsProvider
	Inventory InventoryProvider
	Options   SimulationOptions
}
