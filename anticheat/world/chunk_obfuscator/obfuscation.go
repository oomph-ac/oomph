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

// HasCandidates reports whether the chunk, or any selected layer, contains blocks eligible for obfuscation.
func (x *Obfuscator) HasCandidates(c *chunk.Chunk, dimensionID int32, layers ...int) bool {
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

// Obfuscate obfuscates the chunk, or only the selected layers when supplied.
func (x *Obfuscator) Obfuscate(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64, layers ...int) int {
	d := x.dimension(dimensionID)
	if !x.enabled || d == nil || !d.enabled {
		return 0
	}

	candidates := d.candidates()
	if candidates == 0 {
		return 0
	}

	return obfuscateChunk(c, neighbors, d, candidates, seed, layers)
}

func obfuscateChunk(c *chunk.Chunk, neighbors NeighborChunks, d *dimension, candidates blockType, seed uint64, layers []int) int {
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
	fromY, toY := max(minY, subMinY), min(maxY, subMinY+15)
	if d.mode == oconfig.ObfuscationModeHide {
		return obfuscateHiddenBlocks(storage, d, candidates, fromY, toY, minX, maxX, minZ, maxZ)
	}
	return obfuscateEnclosedBlocks(c, neighbors, storage, d, candidates, seed, fromY, toY, minX, maxX, minZ, maxZ)
}

func obfuscateHiddenBlocks(storage *chunk.PalettedStorage, d *dimension, candidates blockType, minY, maxY int, minX, maxX, minZ, maxZ byte) int {
	changed := 0
	for y := minY; y <= maxY; y++ {
		replacement := d.hideReplacement(y)
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				runtimeID := storage.At(x, byte(y), z)
				if !d.has(runtimeID, candidates) {
					continue
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

func obfuscateEnclosedBlocks(c *chunk.Chunk, neighbors NeighborChunks, storage *chunk.PalettedStorage, d *dimension, candidates blockType, seed uint64, minY, maxY int, minX, maxX, minZ, maxZ byte) int {
	palette := storage.Palette()
	var paletteClasses [4096]blockType
	for index := range palette.Len() {
		runtimeID := palette.Value(uint16(index))
		if int(runtimeID) < len(d.blocks) {
			paletteClasses[index] = d.blocks[runtimeID]
		}
	}

	var paletteIndexes [4096]uint16
	var blockClasses [4096]blockType
	for x := byte(0); x < 16; x++ {
		for z := byte(0); z < 16; z++ {
			for y := byte(0); y < 16; y++ {
				offset := int(x)<<8 | int(z)<<4 | int(y)
				paletteIndex := storage.PaletteIndex(x, y, z)
				paletteIndexes[offset] = paletteIndex
				blockClasses[offset] = paletteClasses[paletteIndex]
			}
		}
	}

	changed := 0
	for y := minY; y <= maxY; y++ {
		layerBlock := uint32(0)
		if d.mode == oconfig.ObfuscationModeLayered {
			layerBlock = d.decoy(seed ^ uint64(int64(y)))
		}
		for x := minX; x <= maxX; x++ {
			xOffset := int(x)<<8 | int(byte(y)&15)
			for z := minZ; z <= maxZ; z++ {
				offset := xOffset | int(z)<<4
				if blockClasses[offset]&candidates == 0 || !d.enclosedInCache(c, neighbors, &blockClasses, offset, x, int16(y), z) {
					continue
				}

				runtimeID := palette.Value(paletteIndexes[offset])
				replacement := layerBlock
				if d.mode == oconfig.ObfuscationModeRandom {
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

func (d dimension) enclosedInCache(c *chunk.Chunk, neighbors NeighborChunks, classes *[4096]blockType, offset int, x byte, y int16, z byte) bool {
	switch {
	case x == 0 && (neighbors.West == nil || !d.isSolid(neighbors.West.Block(15, y, z, 0))):
		return false
	case x != 0 && classes[offset-256] == 0:
		return false
	case x == 15 && (neighbors.East == nil || !d.isSolid(neighbors.East.Block(0, y, z, 0))):
		return false
	case x != 15 && classes[offset+256] == 0:
		return false
	case z == 0 && (neighbors.North == nil || !d.isSolid(neighbors.North.Block(x, y, 15, 0))):
		return false
	case z != 0 && classes[offset-16] == 0:
		return false
	case z == 15 && (neighbors.South == nil || !d.isSolid(neighbors.South.Block(x, y, 0, 0))):
		return false
	case z != 15 && classes[offset+16] == 0:
		return false
	}

	switch byte(y) & 15 {
	case 0:
		return d.isSolid(c.Block(x, y-1, z, 0)) && classes[offset+1] != 0
	case 15:
		return classes[offset-1] != 0 && d.isSolid(c.Block(x, y+1, z, 0))
	default:
		return classes[offset-1] != 0 && classes[offset+1] != 0
	}
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

func (d dimension) enclosedInStorage(c *chunk.Chunk, neighbors NeighborChunks, storage *chunk.PalettedStorage, x byte, y int16, z byte) bool {
	localY := byte(y)
	interior := x != 0 && x != 15 && z != 0 && z != 15
	if interior && (!d.isSolid(storage.At(x-1, localY, z)) || !d.isSolid(storage.At(x+1, localY, z)) || !d.isSolid(storage.At(x, localY, z-1)) || !d.isSolid(storage.At(x, localY, z+1))) {
		return false
	}

	if !interior && !d.edgeNeighborsSolid(neighbors, storage, x, y, z) {
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

func (d dimension) edgeNeighborsSolid(neighbors NeighborChunks, storage *chunk.PalettedStorage, x byte, y int16, z byte) bool {
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
