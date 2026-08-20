package chunkobfuscator

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/oconfig"
)

func compileDimension(registry chunk.BlockRegistry, opts oconfig.ChunkObfuscatorDimensionOpts) (dimension, error) {
	if !opts.Enabled {
		return dimension{}, nil
	}

	switch opts.Mode {
	case oconfig.ObfuscationModeHide, oconfig.ObfuscationModeRandom, oconfig.ObfuscationModeLayered:
	default:
		return dimension{}, fmt.Errorf("unknown mode %q", opts.Mode)
	}

	if len(opts.HiddenBlocks) == 0 {
		return dimension{}, fmt.Errorf("hidden blocks cannot be empty")
	}

	classes := make(map[string]blockType, len(opts.HiddenBlocks)+len(opts.TerrainBlocks))
	for _, name := range opts.HiddenBlocks {
		classes[name] |= hiddenBlock
	}
	for _, name := range opts.TerrainBlocks {
		classes[name] |= terrainBlock
	}

	d := dimension{enabled: opts.Enabled, mode: opts.Mode, maxY: opts.MaxY, blocks: make([]blockType, registry.BlockCount()), decoys: make([]uint32, 0, len(opts.HiddenBlocks))}
	for runtimeID := range d.blocks {
		name, _, ok := registry.RuntimeIDToState(uint32(runtimeID))
		if ok {
			d.blocks[runtimeID] = classes[name]
		}
	}
	for _, name := range opts.HiddenBlocks {
		runtimeID, err := blockRuntimeID(registry, name)
		if err != nil {
			return dimension{}, fmt.Errorf("hidden block: %w", err)
		}
		d.decoys = append(d.decoys, runtimeID)
	}
	for _, name := range opts.TerrainBlocks {
		if _, err := blockRuntimeID(registry, name); err != nil {
			return dimension{}, fmt.Errorf("terrain block: %w", err)
		}
	}
	replacement, err := blockRuntimeID(registry, opts.ReplacementBlock)
	if err != nil {
		return dimension{}, fmt.Errorf("replacement block: %w", err)
	}
	deepReplacement, err := blockRuntimeID(registry, opts.DeepReplacementBlock)
	if err != nil {
		return dimension{}, fmt.Errorf("deep replacement block: %w", err)
	}
	d.replacement, d.deepReplacement = replacement, deepReplacement
	return d, nil
}

func blockRuntimeID(registry chunk.BlockRegistry, name string) (uint32, error) {
	runtimeID, ok := registry.StateToRuntimeID(name, nil)
	if !ok {
		return 0, fmt.Errorf("unknown block %q", name)
	}
	return runtimeID, nil
}

func (d dimension) has(runtimeID uint32, candidates blockType) bool {
	return int(runtimeID) < len(d.blocks) && d.blocks[runtimeID]&candidates != 0
}

func (d dimension) candidates() blockType {
	switch d.mode {
	case oconfig.ObfuscationModeHide:
		return hiddenBlock
	case oconfig.ObfuscationModeRandom, oconfig.ObfuscationModeLayered:
		return hiddenBlock | terrainBlock
	default:
		return 0
	}
}

func (d dimension) isSolid(runtimeID uint32) bool {
	return int(runtimeID) < len(d.blocks) && d.blocks[runtimeID] != 0
}

func (d dimension) paletteContains(storage *chunk.PalettedStorage, candidates blockType) bool {
	palette := storage.Palette()
	for index := range palette.Len() {
		if d.has(palette.Value(uint16(index)), candidates) {
			return true
		}
	}
	return false
}
