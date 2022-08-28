package check

import (
	"github.com/oomph-ac/oomph/entity"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

// Processor represents a check processor, which can be used to process certain checks.
type Processor interface {
	// ServerTick returns the current "server" tick of the processor.
	ServerTick() uint64
	// ClientTick returns the current client tick of the processor
	ClientTick() uint64
	// ClientFrame returns the current simulation frame of the processor.
	ClientFrame() uint64

	// IdentityData returns the login.IdentityData of a processor. It contains the UUID, XUID and username of the connection.
	IdentityData() login.IdentityData
	// ClientData returns the login.ClientData of a processor. This includes less sensitive data of the processor like its skin,
	// language code and other non-essential information.
	ClientData() login.ClientData

	// SearchEntity queries the processor for an entity, using the runtime ID specified. The second return value is false
	// if the entity is not loaded inside the processor memory.
	SearchEntity(rid uint64) (*entity.Entity, bool)
	// Entity returns the entity data of the processor.
	Entity() *entity.Entity

	// ClickDelay returns the delay between the current click and the last one.
	ClickDelay() uint64
	// Click adds a click to the processor's click history.
	Click()
	// CPS returns the clicks per second of the processor.
	CPS() int

	// Ready returns true if the processor is ready/spawned in.
	Ready() bool

	// GameMode returns the current game mode of the processor.
	GameMode() int32
	// Sneaking returns true if the processor is currently sneaking.
	Sneaking() bool
	// Sprinting returns true if the processor is currently sprinting.
	Sprinting() bool
	// Teleporting returns true if the processor is currently teleporting.
	Teleporting() bool
	// Jumping returns true if the processor is currently jumping.
	Jumping() bool
	// Immobile returns true if the processor is currently immobile.
	Immobile() bool
	// Flying returns true if the processor is currently flying.
	Flying() bool
	// Dead returns true if the processor is currently dead.
	Dead() bool
	// Clicking returns true if the processor is clicking.
	Clicking() bool

	// Debug debugs the given parameters to the processor.
	Debug(check Check, params map[string]any)
	// Flag flags the given check with the given parameters.
	Flag(check Check, violations float64, params map[string]any)

	SendOomphDebug(message string)
}
