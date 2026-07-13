package world

import "github.com/df-mc/dragonfly/server/world/chunk"

// NetworkBlockIDToRuntimeID converts a block network ID received over the network into a dragonfly runtime ID.
// When useHashes is true, the network ID is a block state hash (as negotiated via the StartGame
// UseBlockNetworkIDHashes field) and is resolved to its runtime ID. Otherwise the network ID is already a
// runtime ID and is returned unchanged. Unknown hashes resolve to air.
func NetworkBlockIDToRuntimeID(networkID uint32, useHashes bool) uint32 {
	if !useHashes {
		return networkID
	}
	if rid, ok := BlockRegistry.HashToRuntimeID(networkID); ok {
		return rid
	}
	return AirRuntimeID
}

// RuntimeIDToNetworkBlockID converts a dragonfly runtime ID into the block network ID that should be written
// to a connection. When useHashes is true, the runtime ID is converted to its block state hash. Otherwise the
// runtime ID is returned unchanged. Runtime IDs without a known hash are returned unchanged.
func RuntimeIDToNetworkBlockID(rid uint32, useHashes bool) uint32 {
	if !useHashes {
		return rid
	}
	if hash, ok := BlockRegistry.RuntimeIDToHash(rid); ok {
		return hash
	}
	return rid
}

// convertHashPalettesToRuntimeIDs rewrites every block palette in the chunk so that block state hashes are
// replaced with their corresponding dragonfly runtime IDs. It is used when a chunk is decoded from a connection
// that uses block network ID hashes rather than runtime IDs.
func convertHashPalettesToRuntimeIDs(c *chunk.Chunk) {
	for _, sub := range c.Sub() {
		if sub == nil {
			continue
		}
		convertSubChunkHashPalettes(sub)
	}
}

// convertSubChunkHashPalettes rewrites every block palette in the sub chunk, replacing block state hashes with
// their corresponding dragonfly runtime IDs.
func convertSubChunkHashPalettes(sub *chunk.SubChunk) {
	for _, storage := range sub.Layers() {
		if storage == nil {
			continue
		}
		palette := storage.Palette()
		if palette == nil {
			continue
		}
		palette.Replace(func(v uint32) uint32 {
			return NetworkBlockIDToRuntimeID(v, true)
		})
	}
}
