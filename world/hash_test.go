package world

import (
	"bytes"
	"testing"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestNetworkBlockIDHashRoundTrip(t *testing.T) {
	FinalizeBlockRegistry()

	// Without hashes, network IDs are runtime IDs and pass through unchanged.
	if got := NetworkBlockIDToRuntimeID(AirRuntimeID, false); got != AirRuntimeID {
		t.Fatalf("expected passthrough %d, got %d", AirRuntimeID, got)
	}
	if got := RuntimeIDToNetworkBlockID(AirRuntimeID, false); got != AirRuntimeID {
		t.Fatalf("expected passthrough %d, got %d", AirRuntimeID, got)
	}

	// With hashes, a runtime ID converts to a hash and back to the same runtime ID.
	for _, rid := range []uint32{AirRuntimeID, 1, 100, 1000} {
		hash := RuntimeIDToNetworkBlockID(rid, true)
		back := NetworkBlockIDToRuntimeID(hash, true)
		if back != rid {
			t.Fatalf("round trip failed for rid %d: hash=%d back=%d", rid, hash, back)
		}
	}
}

// TestCacheChunkWithHashes verifies that a chunk encoded with block state hashes in its palette is decoded back
// into a chunk whose blocks resolve to the correct dragonfly runtime IDs.
func TestCacheChunkWithHashes(t *testing.T) {
	FinalizeBlockRegistry()

	stoneRID, ok := BlockRegistry.StateToRuntimeID("minecraft:stone", nil)
	if !ok {
		t.Fatal("could not resolve stone runtime ID")
	}

	rng := world.Overworld.Range()
	c := chunk.New(BlockRegistry, rng)
	c.SetBlock(0, int16(rng[0]), 0, 0, stoneRID)
	c.SetBlock(5, int16(rng[0])+7, 3, 0, stoneRID)

	// Rewrite the palettes so the encoded payload carries block state hashes rather than runtime IDs, exactly
	// as a server with UseBlockNetworkIDHashes enabled would send them.
	for _, sub := range c.Sub() {
		if sub == nil {
			continue
		}
		for _, storage := range sub.Layers() {
			if storage == nil || storage.Palette() == nil {
				continue
			}
			storage.Palette().Replace(func(v uint32) uint32 {
				return RuntimeIDToNetworkBlockID(v, true)
			})
		}
	}

	serialised := chunk.Encode(c, chunk.NetworkEncoding)
	buf := bytes.NewBuffer(nil)
	for _, sub := range serialised.SubChunks {
		buf.Write(sub)
	}
	buf.Write(serialised.Biomes)
	buf.WriteByte(0)

	pk := &packet.LevelChunk{
		Dimension:     0,
		SubChunkCount: uint32(len(serialised.SubChunks)),
		RawPayload:    buf.Bytes(),
	}

	info, err := CacheChunk(pk, true)
	if err != nil {
		t.Fatalf("CacheChunk failed: %v", err)
	}

	if got := info.Chunk.Block(0, int16(rng[0]), 0, 0); got != stoneRID {
		t.Fatalf("expected stone rid %d at origin, got %d", stoneRID, got)
	}
	if got := info.Chunk.Block(5, int16(rng[0])+7, 3, 0); got != stoneRID {
		t.Fatalf("expected stone rid %d, got %d", stoneRID, got)
	}
	if got := info.Chunk.Block(1, int16(rng[0]), 0, 0); got != AirRuntimeID {
		t.Fatalf("expected air rid %d for empty block, got %d", AirRuntimeID, got)
	}
}
