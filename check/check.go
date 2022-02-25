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

// registeredChecks maps the name of a check to the check itself.
var registeredChecks = make(map[string]Check)

// RegisterCheck registers the provided check.
func RegisterCheck(check Check) {
	name, suffix := check.Name()
	registeredChecks[name+suffix] = check
}

// Checks returns a slice of all registered checks.
func Checks() []Check {
	checks := make([]Check, 0, len(registeredChecks))
	for _, check := range registeredChecks {
		checks = append(checks, check)
	}
	return checks
}

// FilteredChecks returns a slice of all registered checks that match the provided filter.
func FilteredChecks(filter string) []Check {
	checks := make([]Check, 0, len(registeredChecks))
	for name, check := range registeredChecks {
		if name == filter {
			checks = append(checks, check)
		}
	}
	return checks
}
