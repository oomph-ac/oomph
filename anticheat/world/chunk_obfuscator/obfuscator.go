package chunkobfuscator

import (
	"fmt"
	"sync"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/oconfig"
	"github.com/oomph-ac/oomph/anticheat/oerror"
	oworld "github.com/oomph-ac/oomph/anticheat/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	hiddenBlock blockType = 1 << iota
	terrainBlock
)

type blockType uint8

type Obfuscator struct {
	enabled     bool
	blockRadius int
	overworld   dimension
	nether      dimension
}

type dimension struct {
	enabled         bool
	mode            oconfig.ObfuscationMode
	maxY            int
	blocks          []blockType
	decoys          []uint32
	replacement     uint32
	deepReplacement uint32
}

var (
	current  *Obfuscator
	initOnce sync.Once
)

func newObfuscator(registry chunk.BlockRegistry, opts oconfig.ChunkObfuscatorOpts) (*Obfuscator, error) {
	if opts.BlockRadius < 0 || opts.BlockRadius > 4 {
		return nil, fmt.Errorf("block radius must be between 0 and 4")
	}
	obfuscator := &Obfuscator{enabled: opts.Enabled, blockRadius: opts.BlockRadius}
	if !opts.Enabled {
		return obfuscator, nil
	}
	overworld, err := compileDimension(registry, opts.Dimensions.Overworld)
	if err != nil {
		return nil, fmt.Errorf("overworld: %w", err)
	}
	nether, err := compileDimension(registry, opts.Dimensions.Nether)
	if err != nil {
		return nil, fmt.Errorf("nether: %w", err)
	}
	obfuscator.overworld, obfuscator.nether = overworld, nether
	return obfuscator, nil
}

func Init() {
	initOnce.Do(func() {
		obfuscator, err := newObfuscator(oworld.BlockRegistry, oconfig.ChunkObfuscator())
		if err != nil {
			panic(oerror.New("unable to initialize chunk obfuscator: %v", err))
		}
		current = obfuscator
	})
}

func Current() *Obfuscator { return current }

func (x *Obfuscator) Enabled(dimensionID int32) bool {
	d := x.dimension(dimensionID)
	return x.enabled && d != nil && d.enabled
}

func (x *Obfuscator) BlockRadius() int { return x.blockRadius }

func (x *Obfuscator) ExposesBlocks(oldRuntimeID, newRuntimeID uint32, dimensionID int32) bool {
	d := x.dimension(dimensionID)
	return x.enabled && d != nil && d.enabled && d.isSolid(oldRuntimeID) && !d.isSolid(newRuntimeID)
}

func (x *Obfuscator) ObfuscatesBlock(runtimeID uint32, dimensionID int32) bool {
	d := x.dimension(dimensionID)
	return x.enabled && d != nil && d.enabled && d.has(runtimeID, d.candidates())
}

func (x *Obfuscator) dimension(dimensionID int32) *dimension {
	switch dimensionID {
	case packet.DimensionOverworld:
		return &x.overworld
	case packet.DimensionNether:
		return &x.nether
	default:
		return nil
	}
}
