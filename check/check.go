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

	// TrackViolation will increment the violations on the check by one.
	TrackViolation()
	// Violations will return the violations the check has currently tracked.
	Violations() float64
	// MaxViolations will return the maximum violations the check can track.
	MaxViolations() float64

	// Process will process the packet provided for the check.
	Process(processor Processor, pk packet.Packet)
}
