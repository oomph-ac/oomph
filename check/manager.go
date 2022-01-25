package check

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// checks is a map of packet ids to a list of checks that require those packets.
var checks = map[uint32][]Check{
	packet.IDPlayerAuthInput: {Timer{}},
}
