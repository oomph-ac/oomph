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
	BottomEdge
	TopEdge
)

type BlockChange struct {
	RuntimeID uint32
	Y         int16
	X         byte
	Z         byte
}

func (x *Obfuscator) EdgesEnabled(dimensionID int32) bool {
	d := x.dimension(dimensionID)
	return x.enabled && d != nil && d.enabled && d.mode != oconfig.ObfuscationModeHide && d.candidates() != 0
}

// EdgeChanges returns updates for the chunk edge, optionally limited to selected layers.
func (x *Obfuscator) EdgeChanges(c *chunk.Chunk, neighbors NeighborChunks, dimensionID int32, seed uint64, edge Edge, layers ...int) []BlockChange {
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
	capacity := edgeCapacity(c, d, candidates, edge, layers, minY, maxY, minX, maxX, minZ, maxZ)
	if capacity == 0 {
		return nil
	}

	changes := make([]BlockChange, 0, capacity)
	if layers == nil {
		for index, sub := range c.Sub() {
			changes = appendLayerEdgeChanges(changes, c, neighbors, d, candidates, seed, edge, index, sub, minY, maxY, minX, maxX, minZ, maxZ)
		}
		return changes
	}

	for _, index := range layers {
		if index >= 0 && index < len(c.Sub()) {
			changes = appendLayerEdgeChanges(changes, c, neighbors, d, candidates, seed, edge, index, c.Sub()[index], minY, maxY, minX, maxX, minZ, maxZ)
		}
	}
	return changes
}

func edgeCapacity(c *chunk.Chunk, d *dimension, candidates blockType, edge Edge, layers []int, minY, maxY int, minX, maxX, minZ, maxZ byte) int {
	if minY > maxY {
		return 0
	}
	area := int(maxX-minX+1) * int(maxZ-minZ+1)
	if layers == nil {
		capacity := 0
		for index, sub := range c.Sub() {
			if layerHasCandidates(c, d, candidates, index, sub, minY, maxY) {
				fromY, toY, ok := edgeYRange(c, edge, index, minY, maxY)
				if ok {
					capacity += (toY - fromY + 1) * area
				}
			}
		}
		return capacity
	}

	capacity := 0
	for _, layer := range layers {
		if layer < 0 || layer >= len(c.Sub()) {
			continue
		}
		if layerHasCandidates(c, d, candidates, layer, c.Sub()[layer], minY, maxY) {
			fromY, toY, ok := edgeYRange(c, edge, layer, minY, maxY)
			if ok {
				capacity += (toY - fromY + 1) * area
			}
		}
	}
	return capacity
}

func edgeYRange(c *chunk.Chunk, edge Edge, layer, minY, maxY int) (fromY, toY int, ok bool) {
	subMinY := ((c.Range().Min() >> 4) + layer) << 4
	fromY, toY = max(minY, subMinY), min(maxY, subMinY+15)
	switch edge {
	case BottomEdge:
		fromY, toY = subMinY, subMinY
	case TopEdge:
		fromY, toY = subMinY+15, subMinY+15
	}

	return fromY, toY, fromY >= minY && toY <= maxY && fromY <= toY
}

func appendLayerEdgeChanges(changes []BlockChange, c *chunk.Chunk, neighbors NeighborChunks, d *dimension, candidates blockType, seed uint64, edge Edge, index int, sub *chunk.SubChunk, minY, maxY int, minX, maxX, minZ, maxZ byte) []BlockChange {
	layers := sub.Layers()
	if sub.Empty() || len(layers) == 0 || !d.paletteContains(layers[0], candidates) {
		return changes
	}

	storage := layers[0]
	fromY, toY, ok := edgeYRange(c, edge, index, minY, maxY)
	if !ok {
		return changes
	}

	for y := fromY; y <= toY; y++ {
		layerBlock := uint32(0)
		if d.mode == oconfig.ObfuscationModeLayered {
			layerBlock = d.decoy(seed ^ uint64(int64(y)))
		}
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				runtimeID := storage.At(x, byte(y), z)
				if !d.has(runtimeID, candidates) || !d.enclosedInStorage(c, neighbors, storage, x, int16(y), z) {
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
	case BottomEdge, TopEdge:
		ok = true
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
	return
}
