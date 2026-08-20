package component

import (
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/game"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component/acknowledgement"
	oworld "github.com/oomph-ac/oomph/anticheat/world"
	"github.com/oomph-ac/oomph/anticheat/world/chunk_obfuscator"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type chunkObfuscator struct {
	player     *player.Player
	obfuscator *chunkobfuscator.Obfuscator
}

type adjacentChunk struct {
	position protocol.ChunkPos
	edge     chunkobfuscator.Edge
}

type subChunkUpdates map[protocol.SubChunkPos][]protocol.BlockChangeEntry

type subChunkObfuscation struct {
	source     *chunk.Chunk
	obfuscated *chunk.Chunk
	layers     []int
}

func (o *subChunkObfuscation) addLayer(layer int) {
	for _, existing := range o.layers {
		if existing == layer {
			return
		}
	}

	o.layers = append(o.layers, layer)
}

func newChunkObfuscator(p *player.Player) *chunkObfuscator {
	return &chunkObfuscator{player: p, obfuscator: chunkobfuscator.Current()}
}

func (x *chunkObfuscator) enabled(dimension int32) bool {
	return x.obfuscator != nil && x.obfuscator.Enabled(dimension)
}

func (x *chunkObfuscator) obfuscateSubChunks(pk *packet.SubChunk, results []acknowledgement.SubChunkUpdateResult) bool {
	if len(results) == 0 || !x.enabled(pk.Dimension) {
		return false
	}

	chunks := make(map[protocol.ChunkPos]*subChunkObfuscation)
	for _, result := range results {
		work, found := chunks[result.Position]
		if !found {
			work = &subChunkObfuscation{source: x.player.World().Chunk(result.Position)}
			chunks[result.Position] = work
		}
		work.addLayer(result.Layer)
	}

	for pos, work := range chunks {
		if work.source == nil || !x.obfuscator.HasCandidatesInLayers(work.source, pk.Dimension, work.layers) {
			continue
		}
		obfuscated := work.source.Clone()
		if x.obfuscator.ObfuscateLayers(obfuscated, x.neighbors(pos), pk.Dimension, chunkSeed(pos), work.layers) != 0 {
			work.obfuscated = obfuscated
		}
	}

	modified := false
	for _, result := range results {
		obfuscated := chunks[result.Position].obfuscated
		if obfuscated == nil {
			continue
		}
		entry := &pk.SubChunkEntries[result.Entry]
		payload, ok := entry.RawPayload.Value()
		if !ok || result.PayloadOffset < 0 || result.PayloadOffset > len(payload) {
			continue
		}
		encoded, err := oworld.EncodeSubChunk(obfuscated, result.Layer, x.player.BlockNetwork())
		if err != nil {
			x.player.Log().Warn("unable to encode obfuscated subchunk", "error", err)
			continue
		}
		entry.RawPayload = protocol.Option(append(encoded, payload[result.PayloadOffset:]...))
		modified = true
	}

	for pos, work := range chunks {
		x.refreshEdges(pos, pk.Dimension, work.layers)
	}

	return modified
}

func (x *chunkObfuscator) obfuscateLevelChunk(pk *packet.LevelChunk, info oworld.ChunkInfo) bool {
	if !x.enabled(pk.Dimension) {
		return false
	}

	if !x.obfuscator.HasCandidates(info.Chunk, pk.Dimension) {
		x.refreshEdges(pk.Position, pk.Dimension, nil)
		return false
	}

	obfuscated := info.Chunk.Clone()
	modified := x.obfuscator.Obfuscate(obfuscated, x.neighbors(pk.Position), pk.Dimension, chunkSeed(pk.Position)) != 0
	if modified {
		if err := oworld.EncodeLevelChunk(pk, obfuscated, info.PayloadOffset, x.player.BlockNetwork()); err != nil {
			x.player.Log().Warn("unable to encode obfuscated chunk", "error", err)
			modified = false
		}
	}
	x.refreshEdges(pk.Position, pk.Dimension, nil)

	return modified
}

func (x *chunkObfuscator) neighbors(pos protocol.ChunkPos) chunkobfuscator.NeighborChunks {
	w := x.player.World()
	return chunkobfuscator.NeighborChunks{West: w.Chunk(protocol.ChunkPos{pos[0] - 1, pos[1]}), East: w.Chunk(protocol.ChunkPos{pos[0] + 1, pos[1]}), North: w.Chunk(protocol.ChunkPos{pos[0], pos[1] - 1}), South: w.Chunk(protocol.ChunkPos{pos[0], pos[1] + 1})}
}

func chunkSeed(pos protocol.ChunkPos) uint64 {
	return uint64(uint32(pos[0]))<<32 | uint64(uint32(pos[1]))
}

func (x *chunkObfuscator) refreshEdges(pos protocol.ChunkPos, dimension int32, layers []int) {
	if !x.enabled(dimension) {
		return
	}

	for _, neighbor := range []adjacentChunk{
		{protocol.ChunkPos{pos[0] - 1, pos[1]}, chunkobfuscator.EastEdge},
		{protocol.ChunkPos{pos[0] + 1, pos[1]}, chunkobfuscator.WestEdge},
		{protocol.ChunkPos{pos[0], pos[1] - 1}, chunkobfuscator.SouthEdge},
		{protocol.ChunkPos{pos[0], pos[1] + 1}, chunkobfuscator.NorthEdge},
	} {
		source := x.player.World().Chunk(neighbor.position)
		if source == nil {
			continue
		}
		changes := x.obfuscator.EdgeChangesForLayers(source, x.neighbors(neighbor.position), dimension, chunkSeed(neighbor.position), neighbor.edge, layers)
		x.sendEdgeChanges(neighbor.position, changes)
	}
}

func (x *chunkObfuscator) sendEdgeChanges(chunkPos protocol.ChunkPos, changes []chunkobfuscator.BlockChange) {
	if len(changes) == 0 {
		return
	}

	x.send(x.edgeUpdates(chunkPos, changes))
}

func (x *chunkObfuscator) edgeUpdates(chunkPos protocol.ChunkPos, changes []chunkobfuscator.BlockChange) subChunkUpdates {
	updates := make(subChunkUpdates)
	for _, change := range changes {
		pos := protocol.BlockPos{chunkPos[0]<<4 + int32(change.X), int32(change.Y), chunkPos[1]<<4 + int32(change.Z)}
		x.addUpdate(updates, pos, change.RuntimeID)
	}
	return updates
}

func (x *chunkObfuscator) exposesBlocks(oldRuntimeID, newRuntimeID uint32) bool {
	dimension := x.player.Dimension()
	return x.enabled(dimension) && x.obfuscator.ExposesBlocks(oldRuntimeID, newRuntimeID, dimension)
}

func (x *chunkObfuscator) showAroundBlock(pos protocol.BlockPos) {
	x.showAround([]df_cube.Pos{{int(pos.X()), int(pos.Y()), int(pos.Z())}})
}

func (x *chunkObfuscator) showAround(changed []df_cube.Pos) {
	if !x.enabled(x.player.Dimension()) {
		return
	}
	x.send(x.updatesAround(changed))
}

func (x *chunkObfuscator) updatesAround(changed []df_cube.Pos) subChunkUpdates {
	radius := x.obfuscator.BlockRadius()
	if radius == 0 {
		return nil
	}

	excluded := make(map[df_cube.Pos]struct{}, len(changed))
	for _, pos := range changed {
		excluded[pos] = struct{}{}
	}

	updates := make(subChunkUpdates)
	shown := make(map[df_cube.Pos]struct{})
	for _, pos := range changed {
		for dx := -radius; dx <= radius; dx++ {
			for dy := -radius; dy <= radius; dy++ {
				for dz := -radius; dz <= radius; dz++ {
					if distance := game.AbsNum(dx) + game.AbsNum(dy) + game.AbsNum(dz); distance == 0 || distance > radius {
						continue
					}
					blockPos := df_cube.Pos{pos[0] + dx, pos[1] + dy, pos[2] + dz}
					if _, found := excluded[blockPos]; found {
						continue
					}
					if _, found := shown[blockPos]; found {
						continue
					}
					chunkPos := protocol.ChunkPos{int32(blockPos[0]) >> 4, int32(blockPos[2]) >> 4}
					if x.player.World().Chunk(chunkPos) == nil {
						continue
					}
					runtimeID := df_world.BlockRuntimeID(x.player.World().Block(blockPos))
					if !x.obfuscator.ObfuscatesBlock(runtimeID, x.player.Dimension()) {
						continue
					}
					x.addUpdate(updates, protocol.BlockPos{int32(blockPos[0]), int32(blockPos[1]), int32(blockPos[2])}, runtimeID)
					shown[blockPos] = struct{}{}
				}
			}
		}
	}

	return updates
}

func (x *chunkObfuscator) addUpdate(updates subChunkUpdates, pos protocol.BlockPos, runtimeID uint32) {
	subChunkPos := protocol.SubChunkPos{pos.X() >> 4, pos.Y() >> 4, pos.Z() >> 4}
	updates[subChunkPos] = append(updates[subChunkPos], protocol.BlockChangeEntry{BlockPos: pos, BlockRuntimeID: x.player.EncodeBlockRuntimeID(runtimeID), Flags: packet.BlockUpdateNetwork})
}

func (x *chunkObfuscator) send(updates subChunkUpdates) {
	for pos, entries := range updates {
		_ = x.player.SendPacketToClient(&packet.UpdateSubChunkBlocks{Position: protocol.BlockPos{pos[0], pos[1], pos[2]}, Blocks: entries})
	}
}
