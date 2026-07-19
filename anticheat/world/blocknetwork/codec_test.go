package blocknetwork_test

import (
	"math"
	"os"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/world"
	"github.com/oomph-ac/oomph/anticheat/world/blocknetwork"
)

func TestMain(m *testing.M) {
	world.FinalizeBlockRegistry()
	os.Exit(m.Run())
}

func TestModeFromHashes(t *testing.T) {
	t.Parallel()

	if got := blocknetwork.ModeFromHashes(false); got != blocknetwork.RuntimeIDs {
		t.Fatalf("ModeFromHashes(false) = %v, want RuntimeIDs", got)
	}
	if got := blocknetwork.ModeFromHashes(true); got != blocknetwork.Hashes {
		t.Fatalf("ModeFromHashes(true) = %v, want Hashes", got)
	}
}

func TestCodecConvertsNetworkIDs(t *testing.T) {
	t.Parallel()

	runtimeID := world.BlockRegistry.BlockRuntimeID(block.Stone{})
	networkHash, ok := world.BlockRegistry.RuntimeIDToHash(runtimeID)
	if !ok {
		t.Fatal("stone runtime ID has no network hash")
	}

	tests := []struct {
		name      string
		codec     blocknetwork.Codec
		networkID uint32
	}{
		{name: "runtime IDs", codec: blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.RuntimeIDs), networkID: runtimeID},
		{name: "hashes", codec: blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.Hashes), networkID: networkHash},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, found := test.codec.ToRuntimeID(test.networkID); !found || got != runtimeID {
				t.Fatalf("ToRuntimeID(%d) = (%d, %t), want (%d, true)", test.networkID, got, found, runtimeID)
			}
			if got, found := test.codec.FromRuntimeID(runtimeID); !found || got != test.networkID {
				t.Fatalf("FromRuntimeID(%d) = (%d, %t), want (%d, true)", runtimeID, got, found, test.networkID)
			}
			if got := test.codec.Mode(); got != blocknetwork.ModeFromHashes(test.name == "hashes") {
				t.Fatalf("Mode() = %v, unexpected for %s", got, test.name)
			}
		})
	}
}

func TestCodecRejectsUnknownIDs(t *testing.T) {
	t.Parallel()

	runtimeCodec := blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.RuntimeIDs)
	hashCodec := blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.Hashes)
	unknownRuntimeID := uint32(world.BlockRegistry.BlockCount() + 100)
	unknownHash := unknownNetworkHash(t)

	if _, ok := runtimeCodec.ToRuntimeID(unknownRuntimeID); ok {
		t.Fatalf("runtime codec accepted unknown runtime ID %d", unknownRuntimeID)
	}
	if _, ok := runtimeCodec.FromRuntimeID(unknownRuntimeID); ok {
		t.Fatalf("runtime codec encoded unknown runtime ID %d", unknownRuntimeID)
	}
	if _, ok := hashCodec.ToRuntimeID(unknownHash); ok {
		t.Fatalf("hash codec accepted unknown hash %d", unknownHash)
	}
	if _, ok := hashCodec.FromRuntimeID(unknownRuntimeID); ok {
		t.Fatalf("hash codec encoded unknown runtime ID %d", unknownRuntimeID)
	}
}

func TestRuntimeCodecUsesRegistryLookupInsteadOfAssumingDenseIDs(t *testing.T) {
	t.Parallel()

	runtimeID := world.BlockRegistry.BlockRuntimeID(block.Stone{})
	registry := sparseBlockRegistry{BlockRegistry: world.BlockRegistry, missing: runtimeID}
	codec := blocknetwork.NewCodec(registry, blocknetwork.RuntimeIDs)

	if _, ok := codec.ToRuntimeID(runtimeID); ok {
		t.Fatalf("runtime codec accepted missing runtime ID %d", runtimeID)
	}
	if _, ok := codec.FromRuntimeID(runtimeID); ok {
		t.Fatalf("runtime codec encoded missing runtime ID %d", runtimeID)
	}
}

func TestTranslatorConvertsBetweenModes(t *testing.T) {
	t.Parallel()

	runtimeID := world.BlockRegistry.BlockRuntimeID(block.Stone{})
	networkHash, ok := world.BlockRegistry.RuntimeIDToHash(runtimeID)
	if !ok {
		t.Fatal("stone runtime ID has no network hash")
	}
	runtimeCodec := blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.RuntimeIDs)
	hashCodec := blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.Hashes)

	tests := []struct {
		name   string
		source blocknetwork.Codec
		target blocknetwork.Codec
		input  uint32
		want   uint32
		needed bool
	}{
		{name: "runtime to runtime", source: runtimeCodec, target: runtimeCodec, input: runtimeID, want: runtimeID},
		{name: "runtime to hash", source: runtimeCodec, target: hashCodec, input: runtimeID, want: networkHash, needed: true},
		{name: "hash to runtime", source: hashCodec, target: runtimeCodec, input: networkHash, want: runtimeID, needed: true},
		{name: "hash to hash", source: hashCodec, target: hashCodec, input: networkHash, want: networkHash},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			translator := blocknetwork.NewTranslator(test.source, test.target)
			if got := translator.Required(); got != test.needed {
				t.Fatalf("Required() = %t, want %t", got, test.needed)
			}
			if got := translator.Translate(test.input); got != test.want {
				t.Fatalf("Translate(%d) = %d, want %d", test.input, got, test.want)
			}
		})
	}
}

func TestTranslatorPreservesUnknownAndHighBitIDs(t *testing.T) {
	t.Parallel()

	runtimeCodec := blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.RuntimeIDs)
	hashCodec := blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.Hashes)
	unknownHash := unknownNetworkHash(t)
	if got := blocknetwork.NewTranslator(hashCodec, runtimeCodec).Translate(unknownHash); got != unknownHash {
		t.Fatalf("unknown hash translated to %d, want original %d", got, unknownHash)
	}
	unknownRuntimeID := uint32(world.BlockRegistry.BlockCount() + 100)
	if got := blocknetwork.NewTranslator(runtimeCodec, hashCodec).Translate(unknownRuntimeID); got != unknownRuntimeID {
		t.Fatalf("unknown runtime ID translated to %d, want original %d", got, unknownRuntimeID)
	}

	highBitHash, runtimeID := highBitNetworkHash(t)
	if got := blocknetwork.NewTranslator(hashCodec, runtimeCodec).Translate(highBitHash); got != runtimeID {
		t.Fatalf("high-bit hash %#x translated to %d, want runtime ID %d", highBitHash, got, runtimeID)
	}
	if got := blocknetwork.NewTranslator(runtimeCodec, hashCodec).Translate(runtimeID); got != highBitHash {
		t.Fatalf("runtime ID %d translated to %#x, want high-bit hash %#x", runtimeID, got, highBitHash)
	}
}

func TestNewCodecRejectsNilRegistry(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("NewCodec(nil, RuntimeIDs) did not panic")
		}
	}()
	blocknetwork.NewCodec(nil, blocknetwork.RuntimeIDs)
}

func unknownNetworkHash(t *testing.T) uint32 {
	t.Helper()
	for hash := uint32(math.MaxUint32); ; hash-- {
		if _, ok := world.BlockRegistry.HashToRuntimeID(hash); !ok {
			return hash
		}
	}
}

func highBitNetworkHash(t *testing.T) (uint32, uint32) {
	t.Helper()
	for runtimeID := range uint32(world.BlockRegistry.BlockCount()) {
		hash, ok := world.BlockRegistry.RuntimeIDToHash(runtimeID)
		if ok && hash > math.MaxInt32 {
			return hash, runtimeID
		}
	}
	t.Fatal("block registry contains no high-bit network hash")
	return 0, 0
}

type sparseBlockRegistry struct {
	chunk.BlockRegistry
	missing uint32
}

func (r sparseBlockRegistry) RuntimeIDToState(runtimeID uint32) (string, map[string]any, bool) {
	if runtimeID == r.missing {
		return "", nil, false
	}
	return r.BlockRegistry.RuntimeIDToState(runtimeID)
}
