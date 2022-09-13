package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Check represents any form of detection model which can process packets for unexpected behaviour.
type Check interface {
	// Name will return the name of the check, and the type (eg: A, B), return "AutoClicker", "A".
	Name() (string, string)
	// Description is a description of what this check is for.
	Description() string

	// AddViolation will increment the violations by the given amount
	AddViolation(v float64)
	// Violations will return the violations the check has currently tracked.
	Violations() float64
	// MaxViolations will return the maximum violations the check can track.
	MaxViolations() float64

	// Process will process the packet provided for the check. The boolean returned indicates whether or not
	// the packet should not be sent to the server.
	Process(p Processor, pk packet.Packet) bool
}
