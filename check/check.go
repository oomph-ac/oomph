package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/justtaldevelops/oomph/entity"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Processor represents a check processor, which can be used to process certain checks.
type Processor interface {
	// Tick returns the current tick of the processor.
	Tick() uint64

	// Location returns the current location of the processor.
	Location() entity.Location
	// EntityLocation queries the processor for the location of entity, using the runtime ID specified. The second
	// return value is false if the entity is not loaded inside the processor's memory.
	EntityLocation(rid uint64) (entity.Location, bool)

	// Debug debugs the given parameters to the processor.
	Debug(check Check, params ...map[string]interface{})
	// Flag flags the given check with the given parameters.
	Flag(check Check, params ...map[string]interface{})
}

// Check represents any form of detection model which can process packets for unexpected behaviour.
type Check interface {
	// Name will return the name of the check, and the type (eg: A, B), return "AutoClicker", "A".
	Name() (string, string)
	// Description is a description of what this check is for.
	Description() string
	// Violations will return the violations the check has currently tracked.
	Violations() uint32
	// MaxViolations will return the amount of violations before a punishment is issued.
	MaxViolations() uint32
	// TrackViolation will increment the violations on the check by one.
	TrackViolation()
	// Punishment will return the type of punishment to be issued.
	Punishment() punishment.Punishment
	// Process will process the packet provided for the check.
	Process(processor Processor, pk packet.Packet)
}

// check contains common fields utilized by all checks.
type check struct {
	violations uint32
}

// TrackViolation ...
func (t check) TrackViolation() {
	t.violations++
}

// Violations ...
func (t check) Violations() uint32 {
	return t.violations
}
