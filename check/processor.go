package check

import (
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/entity"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

// Processor represents a check processor, which can be used to process certain checks.
type Processor interface {
	// ServerTick returns the current "server" tick of the processor.
	ServerTick() uint64
	// ClientTick returns the current client tick of the processor
	ClientTick() uint64

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

	// ClimbableTicks returns the amount of climbable ticks the processor has.
	ClimbableTicks() uint32
	// CobwebTicks returns the amount of cobweb ticks the processor has.
	CobwebTicks() uint32
	// LiquidTicks returns the amount of liquid ticks the processor has.
	LiquidTicks() uint32
	// MotionTicks returns the amount of motion ticks the processor has.
	MotionTicks() uint32
	// SpawnTicks returns the amount of spawn ticks the processor has.
	SpawnTicks() uint32

	// CollidedVertically returns true if the processor has collided vertically.
	CollidedVertically() bool
	// CollidedHorizontally returns true if the processor has collided horizontally.
	CollidedHorizontally() bool

	// Motion returns the motion of the processor.
	Motion() mgl64.Vec3
	// ServerPredictedMotion returns the server-predicted motion of the processor.
	ServerPredictedMotion() mgl64.Vec3
	// PreviousServerPredictedMotion returns the previous server-predicted motion of the processor.
	PreviousServerPredictedMotion() mgl64.Vec3

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
	Debug(check Check, params map[string]interface{})
	// Flag flags the given check with the given parameters.
	Flag(check Check, violations float64, params map[string]interface{})
}
