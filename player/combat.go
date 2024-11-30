package player

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type CombatHook func(CombatComponent)

// CombatComponent is responsible for managing and simulating combat mechanics for players on the server.
// It ensures that all players operate under the same rules and conditions during combat.
type CombatComponent interface {
	// Hook adds a hook to the combat component so it may utilize the results of this combat component.
	Hook(CombatHook)

	// Attack notifies the combat component of an attack.
	Attack(pk *packet.InventoryTransaction)
	// Calculate calculates the different distances from the attacked entity to the combat component's
	// current position. The data then can be used to validated on other components or detections.
	// This function should be called when the player ticks on PlayerAuthInput.
	Calculate() bool

	// Swing notifies the combat component of when the member player swings their arm.
	Swing()
	// LastSwing returns the last tick of when the member player swung their arm.
	LastSwing() int64

	// Raycasts returns the different distances from the entity to the combat component's eye position
	// from different raycasts compensating for lerped positions on frame.
	Raycasts() []float32
	// Raws returns the different distances from the entity to the combat component's eye position
	// from a calculation of the closest point from the eye position to the bounding box. It compensates
	// for lerped positions on frame.
	Raws() []float32
}

func (p *Player) SetCombat(c CombatComponent) {
	p.combat = c
}

func (p *Player) Combat() CombatComponent {
	return p.combat
}

func (p *Player) SetClientCombat(c CombatComponent) {
	p.clientCombat = c
}

func (p *Player) ClientCombat() CombatComponent {
	return p.clientCombat
}
