package player

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	INTERPOLATED_STEPS = 10
)

// CombatComponent is responsible for managing and simulating combat mechanics for players on the server.
// It ensures that all players operate under the same rules and conditions during combat.
type CombatComponent interface {
	// Attack notifies the combat component of an attack.
	Attack(pk *protocol.UseItemOnEntityTransactionData)
	// Calculate calculates the different distances from the attacked entity to the combat component's
	// current position. The data then can be used to validated on other components or detections.
	Calculate()
	// Raycasts returns the different distances from the entity to the combat component's eye position
	// from different raycasts compensating for lerped positions on frame.
	Raycasts() []float32
	// Raws returns the different distances from the entity to the combat component's eye position
	// from a calculation of the closest point from the eye position to the bounding box. It compensates
	// for lerped positions on frame.
	Raws() []float32
}
