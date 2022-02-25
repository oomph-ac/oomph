package check

import (
	"github.com/justtaldevelops/oomph/entity"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

// Processor represents a check processor, which can be used to process certain checks.
type Processor interface {
	// ServerTick returns the current "server" tick of the processor.
	ServerTick() uint64
	// ClientTick returns the current client tick of the processor
	ClientTick() uint64

	// IdentityData returns the login.IdentityData of a player. It contains the UUID, XUID and username of the connection.
	IdentityData() login.IdentityData
	// ClientData returns the login.ClientData of a player. This includes less sensitive data of the player like its skin,
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

	// GameMode returns the current game mode of the player.
	GameMode() int32
	// Sneaking returns true if the player is currently sneaking.
	Sneaking() bool
	// Sprinting returns true if the player is currently sprinting.
	Sprinting() bool
	// Teleporting returns true if the player is currently teleporting.
	Teleporting() bool
	// Jumping returns true if the player is currently jumping.
	Jumping() bool
	// Immobile returns true if the player is currently immobile.
	Immobile() bool
	// Flying returns true if the player is currently flying.
	Flying() bool
	// Dead returns true if the player is currently dead.
	Dead() bool
	// Clicking returns true if the player is clicking.
	Clicking() bool

	// Debug debugs the given parameters to the processor.
	Debug(check Check, params map[string]interface{})
	// Flag flags the given check with the given parameters.
	Flag(check Check, violations float64, params map[string]interface{})
}
