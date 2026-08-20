package chunkobfuscator

import (
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/oconfig"
)

type Edge uint8

const (
	WestEdge Edge = iota
	EastEdge
	NorthEdge
	SouthEdge
)

type BlockChange struct {
	X         byte
	Y         int16
	Z         byte
	RuntimeID uint32
}

// EdgeChanges ...
func (x *Obfuscator) EdgeChanges(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64, edge Edge) []BlockChange {
	return x.edgeChanges(c, neighbors, dimensionID, seed, edge, nil)
}

func (x *Obfuscator) EdgeChangesForLayers(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64, edge Edge, layers []int) []BlockChange {
	return x.edgeChanges(c, neighbors, dimensionID, seed, edge, layers)
}

func (x *Obfuscator) edgeChanges(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64, edge Edge, layers []int) []BlockChange {
	d := x.dimension(dimensionID)
	if !x.enabled || d == nil || !d.enabled || d.mode == oconfig.ObfuscationModeHide {
		return nil
	}

	candidates := d.candidates()
	if candidates == 0 {
		return nil
	}

	minX, maxX, minZ, maxZ, ok := edgeBounds(neighbors, edge)
	if !ok {
		return nil
	}

	minY, maxY := c.Range().Min()+1, min(d.maxY, c.Range().Max()-1)
	if layers == nil {
		var changes []BlockChange
		for index, sub := range c.Sub() {
			changes = edgeChangesForLayer(changes, c, neighbors, d, candidates, seed, index, sub, minY, maxY, minX, maxX, minZ, maxZ)
		}
		return changes
	}

	var changes []BlockChange
	for _, index := range layers {
		if index >= 0 && index < len(c.Sub()) {
			changes = edgeChangesForLayer(changes, c, neighbors, d, candidates, seed, index, c.Sub()[index], minY, maxY, minX, maxX, minZ, maxZ)
		}
	}
	return changes
}

func edgeChangesForLayer(changes []BlockChange, c *chunk.Chunk, neighbors NeighborChunks, d *dimension, candidates blockType, seed uint64, index int, sub *chunk.SubChunk, minY, maxY int, minX, maxX, minZ, maxZ byte) []BlockChange {
	layers := sub.Layers()
	if sub.Empty() || len(layers) == 0 || !d.paletteContains(layers[0], candidates) {
		return changes
	}

	storage := layers[0]
	subMinY := ((c.Range().Min() >> 4) + index) << 4
	for y, toY := max(minY, subMinY), min(maxY, subMinY+15); y <= toY; y++ {
		layerBlock := uint32(0)
		if d.mode == oconfig.ObfuscationModeLayered {
			layerBlock = d.decoy(seed ^ uint64(int64(y)))
		}
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				runtimeID := storage.At(x, byte(y), z)
				if !d.has(runtimeID, candidates) || !d.enclosed(c, neighbors, storage, x, int16(y), z) {
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
					changes = append(changes, BlockChange{X: x, Y: int16(y), Z: z, RuntimeID: replacement})
				}
			}
		}
	}

	return changes
}

func edgeBounds(neighbors NeighborChunks, edge Edge) (minX, maxX, minZ, maxZ byte, ok bool) {
	minX, maxX, minZ, maxZ = 1, 14, 1, 14
	switch edge {
	case WestEdge:
		minX, maxX, ok = 0, 0, neighbors.West != nil
		if neighbors.North != nil {
			minZ = 0
		}
		if neighbors.South != nil {
			maxZ = 15
		}
	case EastEdge:
		minX, maxX, ok = 15, 15, neighbors.East != nil
		if neighbors.North != nil {
			minZ = 0
		}
		if neighbors.South != nil {
			maxZ = 15
		}
	case NorthEdge:
		minZ, maxZ, ok = 0, 0, neighbors.North != nil
		if neighbors.West != nil {
			minX = 0
		}
		if neighbors.East != nil {
			maxX = 15
		}
	case SouthEdge:
		minZ, maxZ, ok = 15, 15, neighbors.South != nil
		if neighbors.West != nil {
			minX = 0
		}
		if neighbors.East != nil {
			maxX = 15
		}
	}
	return
}
