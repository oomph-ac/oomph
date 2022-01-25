package check

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

type Check interface {
	// Name will return the name of the check, and the type (eg: A, B), return "AutoClicker", "A".
	Name() (string, string)
	// Description is a description of what this check is for.
	Description() string
	// Punishment will return the type of punishment to be issued.
	Punishment() Punishment
	// Violations will return the max violations allowed before a punishment is issued.
	Violations() uint32
	// Process will process the packet provided for the check.
	Process(pk packet.Packet)
}
