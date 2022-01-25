package check

import (
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Processor represents a check processor, which can be used to process certain checks.
type Processor interface {
	// Tick returns the current tick of the processor.
	Tick() uint64
	// Debug debugs the given parameters to the processor.
	Debug(check Check, params ...map[string]interface{})
	// Flag flags the given check with the given parameters.
	Flag(check Check, params ...map[string]interface{})
}

type Check interface {
	// Name will return the name of the check, and the type (eg: A, B), return "AutoClicker", "A".
	Name() (string, string)
	// Description is a description of what this check is for.
	Description() string
	// Violations will return the violations along with the max violations allowed before a punishment is issued.
	Violations() (uint32, uint32)
	// Track tracks a new violation, adding onto the current violation count. It will return the new violation count,
	// along with the max violations allowed.
	Track() (uint32, uint32)
	// Punishment will return the type of punishment to be issued.
	Punishment() punishment.Punishment
	// Process will process the packet provided for the check.
	Process(processor Processor, pk packet.Packet)
}
