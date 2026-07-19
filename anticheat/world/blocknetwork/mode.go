// Package blocknetwork converts block IDs between Bedrock's runtime-ID and network-hash representations.
package blocknetwork

// Mode identifies the block ID representation used by a network endpoint.
type Mode uint8

const (
	// RuntimeIDs identifies ordinary block-registry runtime IDs.
	RuntimeIDs Mode = iota
	// Hashes identifies block network hashes.
	Hashes
)

// ModeFromHashes returns Hashes when enabled is true and RuntimeIDs otherwise.
func ModeFromHashes(enabled bool) Mode {
	if enabled {
		return Hashes
	}
	return RuntimeIDs
}
