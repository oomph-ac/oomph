package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"math"

	"github.com/justtaldevelops/oomph/entity"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

	// Debug debugs the given parameters to the processor.
	Debug(check Check, params map[string]interface{})
	// Flag flags the given check with the given parameters.
	Flag(check Check, violations float64, params map[string]interface{})
}

// Check represents any form of detection model which can process packets for unexpected behaviour.
type Check interface {
	// Name will return the name of the check, and the type (eg: A, B), return "AutoClicker", "A".
	Name() (string, string)
	// Description is a description of what this check is for.
	Description() string

	// TrackViolation will increment the violations on the check by one.
	TrackViolation()
	// Violations will return the violations the check has currently tracked.
	Violations() float64

	// Process will process the packet provided for the check.
	Process(processor Processor, pk packet.Packet)
}

// check contains common fields utilized by all checks.
type check struct {
	lastFlagTick uint64
	violations   float64
	buffer       float64
}

// Buff adds to the buffer and returns the new one.
func (t *check) Buff(n float64, max ...float64) float64 {
	var m float64 = 15
	if len(max) > 0 {
		m = max[0]
	}
	t.buffer += n
	t.buffer = math.Max(0, t.buffer)
	t.buffer = math.Min(t.buffer, m)
	return t.buffer
}

// TrackViolation ...
func (t *check) TrackViolation() {
	t.violations++
}

// Violations ...
func (t *check) Violations() float64 {
	return t.violations
}

// updateAndGetViolationAfterTicks ...
// TODO: what the fuck is this?
func (t *check) updateAndGetViolationAfterTicks(tick uint64, maxTime float64) float64 {
	defer func() {
		t.lastFlagTick = tick
	}()
	return math.Max((maxTime+math.Min(float64(tick-t.lastFlagTick), 1))/maxTime, 0)
}
