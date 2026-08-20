package chunkobfuscator

import (
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/oconfig"
)

type NeighborChunks struct {
	West  *chunk.Chunk
	East  *chunk.Chunk
	North *chunk.Chunk
	South *chunk.Chunk
}

// Obfuscate ...
func (x *Obfuscator) Obfuscate(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64) int {
	return x.obfuscate(c, neighbors, dimensionID, seed, nil)
}

func (x *Obfuscator) ObfuscateLayers(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64, layers []int) int {
	return x.obfuscate(c, neighbors, dimensionID, seed, layers)
}

func (x *Obfuscator) HasCandidates(c *chunk.Chunk, dimensionID int32) bool {
	return x.hasCandidates(c, dimensionID, nil)
}

func (x *Obfuscator) HasCandidatesInLayers(c *chunk.Chunk, dimensionID int32, layers []int) bool {
	return x.hasCandidates(c, dimensionID, layers)
}

func (x *Obfuscator) hasCandidates(c *chunk.Chunk, dimensionID int32, layers []int) bool {
	d := x.dimension(dimensionID)
	if !x.enabled || d == nil || !d.enabled {
		return false
	}

	candidates := d.candidates()
	minY, maxY := d.bounds(c)
	if candidates == 0 || minY > maxY {
		return false
	}

	if layers == nil {
		for index, sub := range c.Sub() {
			if layerHasCandidates(c, d, candidates, index, sub, minY, maxY) {
				return true
			}
		}
		return false
	}

	for _, index := range layers {
		if index >= 0 && index < len(c.Sub()) && layerHasCandidates(c, d, candidates, index, c.Sub()[index], minY, maxY) {
			return true
		}
	}

	return false
}

func (x *Obfuscator) obfuscate(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64, layers []int) int {
	d := x.dimension(dimensionID)
	if !x.enabled || d == nil || !d.enabled {
		return 0
	}

	candidates := d.candidates()
	if candidates == 0 {
		return 0
	}

	return obfuscate(c, neighbors, d, candidates, seed, layers)
}

func obfuscate(c *chunk.Chunk, neighbors NeighborChunks, d *dimension, candidates blockType, seed uint64, layers []int) int {
	minY, maxY := d.bounds(c)
	if minY > maxY {
		return 0
	}

	minX, maxX, minZ, maxZ := byte(0), byte(15), byte(0), byte(15)
	if d.mode != oconfig.ObfuscationModeHide {
		minX, maxX, minZ, maxZ = 1, 14, 1, 14
		if neighbors.West != nil {
			minX = 0
		}
		if neighbors.East != nil {
			maxX = 15
		}
		if neighbors.North != nil {
			minZ = 0
		}
		if neighbors.South != nil {
			maxZ = 15
		}
	}

	if layers == nil {
		changed := 0
		for index, sub := range c.Sub() {
			changed += obfuscateLayer(c, neighbors, d, candidates, seed, index, sub, minY, maxY, minX, maxX, minZ, maxZ)
		}
		return changed
	}

	changed := 0
	for _, index := range layers {
		if index >= 0 && index < len(c.Sub()) {
			changed += obfuscateLayer(c, neighbors, d, candidates, seed, index, c.Sub()[index], minY, maxY, minX, maxX, minZ, maxZ)
		}
	}

	return changed
}

func obfuscateLayer(c *chunk.Chunk, neighbors NeighborChunks, d *dimension, candidates blockType, seed uint64, index int, sub *chunk.SubChunk, minY, maxY int, minX, maxX, minZ, maxZ byte) int {
	if !layerHasCandidates(c, d, candidates, index, sub, minY, maxY) {
		return 0
	}

	storage := sub.Layers()[0]
	subMinY := ((c.Range().Min() >> 4) + index) << 4
	changed := 0
	for y, toY := max(minY, subMinY), min(maxY, subMinY+15); y <= toY; y++ {
		layerBlock := uint32(0)
		if d.mode == oconfig.ObfuscationModeLayered {
			layerBlock = d.decoy(seed ^ uint64(int64(y)))
		}
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				runtimeID := storage.At(x, byte(y), z)
				if !d.has(runtimeID, candidates) {
					continue
				}
				if d.mode != oconfig.ObfuscationModeHide && !d.enclosed(c, neighbors, storage, x, int16(y), z) {
					continue
				}
				replacement := layerBlock
				switch d.mode {
				case oconfig.ObfuscationModeHide:
					replacement = d.hideReplacement(y)
				case oconfig.ObfuscationModeRandom:
					replacement = d.decoy(blockSeed(seed, x, y, z))
				}
				if replacement != runtimeID {
					storage.Set(x, byte(y), z, replacement)
					changed++
				}
			}
		}
	}

	return changed
}

func layerHasCandidates(c *chunk.Chunk, d *dimension, candidates blockType, index int, sub *chunk.SubChunk, minY, maxY int) bool {
	subMinY := ((c.Range().Min() >> 4) + index) << 4
	layers := sub.Layers()
	return subMinY <= maxY && subMinY+15 >= minY && !sub.Empty() && len(layers) != 0 && d.paletteContains(layers[0], candidates)
}

func (d dimension) bounds(c *chunk.Chunk) (int, int) {
	if d.mode == oconfig.ObfuscationModeHide {
		return c.Range().Min(), min(d.maxY, c.Range().Max())
	}

	return c.Range().Min() + 1, min(d.maxY, c.Range().Max()-1)
}

func (d dimension) enclosed(c *chunk.Chunk, neighbors NeighborChunks, storage *chunk.PalettedStorage, x byte, y int16, z byte) bool {
	localY := byte(y)
	if x != 0 && x != 15 && z != 0 && z != 15 {
		if !d.isSolid(storage.At(x-1, localY, z)) || !d.isSolid(storage.At(x+1, localY, z)) || !d.isSolid(storage.At(x, localY, z-1)) || !d.isSolid(storage.At(x, localY, z+1)) {
			return false
		}
	}

	if (x == 0 || x == 15 || z == 0 || z == 15) && !d.edgeEnclosed(neighbors, storage, x, y, z) {
		return false
	}

	switch localY & 15 {
	case 0:
		return d.isSolid(c.Block(x, y-1, z, 0)) && d.isSolid(storage.At(x, localY+1, z))
	case 15:
		return d.isSolid(storage.At(x, localY-1, z)) && d.isSolid(c.Block(x, y+1, z, 0))
	default:
		return d.isSolid(storage.At(x, localY-1, z)) && d.isSolid(storage.At(x, localY+1, z))
	}
}

func (d dimension) edgeEnclosed(neighbors NeighborChunks, storage *chunk.PalettedStorage, x byte, y int16, z byte) bool {
	localY := byte(y)

	if x == 0 && (neighbors.West == nil || !d.isSolid(neighbors.West.Block(15, y, z, 0))) {
		return false
	}
	if x != 0 && !d.isSolid(storage.At(x-1, localY, z)) {
		return false
	}
	if x == 15 && (neighbors.East == nil || !d.isSolid(neighbors.East.Block(0, y, z, 0))) {
		return false
	}
	if x != 15 && !d.isSolid(storage.At(x+1, localY, z)) {
		return false
	}
	if z == 0 && (neighbors.North == nil || !d.isSolid(neighbors.North.Block(x, y, 15, 0))) {
		return false
	}
	if z != 0 && !d.isSolid(storage.At(x, localY, z-1)) {
		return false
	}
	if z == 15 && (neighbors.South == nil || !d.isSolid(neighbors.South.Block(x, y, 0, 0))) {
		return false
	}
	if z != 15 && !d.isSolid(storage.At(x, localY, z+1)) {
		return false
	}

	return true
}

func (d dimension) hideReplacement(y int) uint32 {
	if y < 0 {
		return d.deepReplacement
	}

	return d.replacement
}

func (d dimension) decoy(value uint64) uint32 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ value>>30) * 0xbf58476d1ce4e5b9
	value = (value ^ value>>27) * 0x94d049bb133111eb
	value ^= value >> 31
	return d.decoys[value%uint64(len(d.decoys))]
}

func blockSeed(seed uint64, x byte, y int, z byte) uint64 {
	return seed ^ uint64(x)<<36 ^ uint64(uint32(y))<<4 ^ uint64(z)
}
