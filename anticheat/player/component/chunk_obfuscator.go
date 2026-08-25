package component

import (
	"slices"

	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/anticheat/game"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component/acknowledgement"
	oworld "github.com/oomph-ac/oomph/anticheat/world"
	chunkobfuscator "github.com/oomph-ac/oomph/anticheat/world/chunk_obfuscator"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type chunkObfuscation struct {
	player    *player.Player
	engine    *chunkobfuscator.Obfuscator
	dimension int32
	pending   pendingReveals
}

type subChunkUpdates map[protocol.SubChunkPos][]protocol.BlockChangeEntry

type subChunkWork struct {
	source     *chunk.Chunk
	obfuscated *chunk.Chunk
	layers     []int
}

type pendingReveals map[protocol.SubChunkPos]map[df_cube.Pos]struct{}

func (r pendingReveals) add(pos df_cube.Pos) {
	subChunkPos := protocol.SubChunkPos{int32(pos[0]) >> 4, int32(pos[1]) >> 4, int32(pos[2]) >> 4}
	if r[subChunkPos] == nil {
		r[subChunkPos] = make(map[df_cube.Pos]struct{})
	}

	r[subChunkPos][pos] = struct{}{}
}

func (r pendingReveals) restore(source, obfuscated *chunk.Chunk, pos protocol.ChunkPos, layers []int) int {
	if len(r) == 0 {
		return 0
	}

	restoreLayer := func(layer int) int {
		if layer < 0 || layer >= len(source.Sub()) {
			return 0
		}

		subChunkPos := protocol.SubChunkPos{pos[0], int32(source.Range().Min()>>4 + layer), pos[1]}
		restored := 0
		for blockPos := range r[subChunkPos] {
			x, y, z := uint8(blockPos[0]), int16(blockPos[1]), uint8(blockPos[2])
			runtimeID := source.Block(x, y, z, 0)
			if obfuscated.Block(x, y, z, 0) != runtimeID {
				obfuscated.SetBlock(x, y, z, 0, runtimeID)
				restored++
			}
		}
		delete(r, subChunkPos)
		return restored
	}
	if layers == nil {
		restored := 0
		for layer := range source.Sub() {
			restored += restoreLayer(layer)
		}
		return restored
	}

	restored := 0
	for _, layer := range layers {
		restored += restoreLayer(layer)
	}
	return restored
}

func (r pendingReveals) consume(source *chunk.Chunk, pos protocol.ChunkPos, layers []int) {
	if layers == nil {
		for layer := range source.Sub() {
			delete(r, protocol.SubChunkPos{pos[0], int32(source.Range().Min()>>4 + layer), pos[1]})
		}
		return
	}

	for _, layer := range layers {
		if layer >= 0 && layer < len(source.Sub()) {
			delete(r, protocol.SubChunkPos{pos[0], int32(source.Range().Min()>>4 + layer), pos[1]})
		}
	}
}

func cloneChunkLayers(source *chunk.Chunk, layers []int) *chunk.Chunk {
	cloned := chunk.New(oworld.BlockRegistry, source.Range())
	copy(cloned.Sub(), source.Sub())
	for _, layer := range layers {
		if layer >= 0 && layer < len(source.Sub()) {
			cloned.Sub()[layer] = source.Sub()[layer].Clone()
		}
	}
	return cloned
}

func newChunkObfuscation(p *player.Player) *chunkObfuscation {
	return &chunkObfuscation{player: p, engine: chunkobfuscator.Current(), dimension: p.Dimension(), pending: make(pendingReveals)}
}

func (x *chunkObfuscation) enabled(dimension int32) bool {
	return x.engine != nil && x.engine.Enabled(dimension)
}

func (x *chunkObfuscation) syncDimension(dimension int32) {
	if x.dimension != dimension {
		x.dimension = dimension
		clear(x.pending)
	}
}

func (x *chunkObfuscation) obfuscateSubChunks(pk *packet.SubChunk, results []acknowledgement.SubChunkUpdateResult) bool {
	x.syncDimension(pk.Dimension)
	if len(results) == 0 || !x.enabled(pk.Dimension) {
		return false
	}

	chunks := make(map[protocol.ChunkPos]*subChunkWork)
	for _, result := range results {
		work, found := chunks[result.Position]
		if !found {
			work = &subChunkWork{source: x.player.World().Chunk(result.Position)}
			chunks[result.Position] = work
		}

		if !slices.Contains(work.layers, result.Layer) {
			work.layers = append(work.layers, result.Layer)
		}
	}

	for pos, work := range chunks {
		if work.source == nil {
			continue
		}

		if !x.engine.HasCandidates(work.source, pk.Dimension, work.layers...) {
			x.pending.consume(work.source, pos, work.layers)
			continue
		}

		obfuscated := cloneChunkLayers(work.source, work.layers)
		changed := x.engine.Obfuscate(obfuscated, x.neighbors(pos), pk.Dimension, chunkSeed(pos), work.layers...)
		changed -= x.pending.restore(work.source, obfuscated, pos, work.layers)
		if changed != 0 {
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
		x.updateEdges(pos, pk.Dimension, work.layers)
	}

	return modified
}

func (x *chunkObfuscation) obfuscateLevelChunk(pk *packet.LevelChunk, info oworld.ChunkInfo) bool {
	x.syncDimension(pk.Dimension)
	if !x.enabled(pk.Dimension) {
		return false
	}

	if !x.engine.HasCandidates(info.Chunk, pk.Dimension) {
		x.pending.consume(info.Chunk, pk.Position, nil)
		x.updateEdges(pk.Position, pk.Dimension, nil)
		return false
	}

	obfuscated := info.Chunk.Clone()
	changed := x.engine.Obfuscate(obfuscated, x.neighbors(pk.Position), pk.Dimension, chunkSeed(pk.Position))
	changed -= x.pending.restore(info.Chunk, obfuscated, pk.Position, nil)
	modified := changed != 0
	if modified {
		if err := oworld.EncodeLevelChunk(pk, obfuscated, info.PayloadOffset, x.player.BlockNetwork()); err != nil {
			x.player.Log().Warn("unable to encode obfuscated chunk", "error", err)
			modified = false
		}
	}

	x.updateEdges(pk.Position, pk.Dimension, nil)

	return modified
}

func (x *chunkObfuscation) neighbors(pos protocol.ChunkPos) chunkobfuscator.NeighborChunks {
	w := x.player.World()
	return chunkobfuscator.NeighborChunks{West: w.Chunk(protocol.ChunkPos{pos[0] - 1, pos[1]}), East: w.Chunk(protocol.ChunkPos{pos[0] + 1, pos[1]}), North: w.Chunk(protocol.ChunkPos{pos[0], pos[1] - 1}), South: w.Chunk(protocol.ChunkPos{pos[0], pos[1] + 1})}
}

func chunkSeed(pos protocol.ChunkPos) uint64 {
	return uint64(uint32(pos[0]))<<32 | uint64(uint32(pos[1]))
}

func (x *chunkObfuscation) updateEdges(pos protocol.ChunkPos, dimension int32, layers []int) {
	if !x.enabled(dimension) || !x.engine.EdgesEnabled(dimension) {
		return
	}

	update := func(target protocol.ChunkPos, edge chunkobfuscator.Edge, targetLayers []int) {
		source := x.player.World().Chunk(target)
		if source == nil {
			return
		}

		if !x.engine.HasCandidates(source, dimension, targetLayers...) {
			return
		}

		changes := x.engine.EdgeChanges(source, x.neighbors(target), dimension, chunkSeed(target), edge, targetLayers...)
		if len(changes) == 0 {
			return
		}

		updates := make(subChunkUpdates)
		for _, change := range changes {
			blockPos := protocol.BlockPos{target[0]<<4 + int32(change.X), int32(change.Y), target[1]<<4 + int32(change.Z)}
			x.addUpdate(updates, blockPos, change.RuntimeID)
		}
		x.send(updates)
	}

	adjacentEdges := [...]struct {
		position protocol.ChunkPos
		edge     chunkobfuscator.Edge
	}{
		{position: protocol.ChunkPos{pos[0] - 1, pos[1]}, edge: chunkobfuscator.EastEdge},
		{position: protocol.ChunkPos{pos[0] + 1, pos[1]}, edge: chunkobfuscator.WestEdge},
		{position: protocol.ChunkPos{pos[0], pos[1] - 1}, edge: chunkobfuscator.SouthEdge},
		{position: protocol.ChunkPos{pos[0], pos[1] + 1}, edge: chunkobfuscator.NorthEdge},
	}
	for _, adjacent := range adjacentEdges {
		update(adjacent.position, adjacent.edge, layers)
	}

	for _, layer := range layers {
		if !slices.Contains(layers, layer-1) {
			update(pos, chunkobfuscator.TopEdge, []int{layer - 1})
		}

		if !slices.Contains(layers, layer+1) {
			update(pos, chunkobfuscator.BottomEdge, []int{layer + 1})
		}
	}
}

func (x *chunkObfuscation) exposesBlocks(oldRuntimeID, newRuntimeID uint32) bool {
	dimension := x.player.Dimension()
	return x.enabled(dimension) && x.engine.ExposesBlocks(oldRuntimeID, newRuntimeID, dimension)
}

func (x *chunkObfuscation) revealAhead(pos df_cube.Pos, face df_cube.Face, depth int) {
	if x.engine == nil {
		return
	}

	positions := blocksAhead(pos, face, depth)
	skip := min(x.engine.BlockRadius(), len(positions))
	if skip == len(positions) {
		return
	}

	x.reveal(positions[skip:])
}

func blocksAhead(pos df_cube.Pos, face df_cube.Face, depth int) []df_cube.Pos {
	if face < df_cube.FaceDown || face > df_cube.FaceEast || depth <= 0 {
		return nil
	}

	positions := make([]df_cube.Pos, 0, depth)
	direction := face.Opposite()
	for range depth {
		pos = pos.Side(direction)
		positions = append(positions, pos)
	}
	return positions
}

func (x *chunkObfuscation) revealAround(changed []df_cube.Pos) {
	dimension := x.player.Dimension()
	x.syncDimension(dimension)
	if !x.enabled(dimension) {
		return
	}

	x.send(x.revealUpdates(revealPositionsAround(changed, x.engine.BlockRadius())))
}

func (x *chunkObfuscation) reveal(positions []df_cube.Pos) {
	dimension := x.player.Dimension()
	x.syncDimension(dimension)
	if !x.enabled(dimension) {
		return
	}

	x.send(x.revealUpdates(positions))
}

func revealPositionsAround(changed []df_cube.Pos, radius int) []df_cube.Pos {
	if radius == 0 || len(changed) == 0 {
		return nil
	}

	volume := (4*radius*radius*radius + 6*radius*radius + 8*radius) / 3
	if len(changed) == 1 {
		positions := make([]df_cube.Pos, 0, volume)
		pos := changed[0]
		for dx := -radius; dx <= radius; dx++ {
			for dy := -radius; dy <= radius; dy++ {
				for dz := -radius; dz <= radius; dz++ {
					if distance := game.AbsNum(dx) + game.AbsNum(dy) + game.AbsNum(dz); distance != 0 && distance <= radius {
						positions = append(positions, df_cube.Pos{pos[0] + dx, pos[1] + dy, pos[2] + dz})
					}
				}
			}
		}
		return positions
	}

	shown := make(map[df_cube.Pos]struct{}, len(changed)*(volume+1))
	for _, pos := range changed {
		shown[pos] = struct{}{}
	}

	positions := make([]df_cube.Pos, 0, len(changed)*volume)
	for _, pos := range changed {
		for dx := -radius; dx <= radius; dx++ {
			for dy := -radius; dy <= radius; dy++ {
				for dz := -radius; dz <= radius; dz++ {
					if distance := game.AbsNum(dx) + game.AbsNum(dy) + game.AbsNum(dz); distance == 0 || distance > radius {
						continue
					}

					blockPos := df_cube.Pos{pos[0] + dx, pos[1] + dy, pos[2] + dz}
					if _, found := shown[blockPos]; found {
						continue
					}

					positions = append(positions, blockPos)
					shown[blockPos] = struct{}{}
				}
			}
		}
	}

	return positions
}

func (x *chunkObfuscation) revealUpdates(positions []df_cube.Pos) subChunkUpdates {
	updates := make(subChunkUpdates)
	dimension := x.player.Dimension()
	w := x.player.World()
	for _, blockPos := range positions {
		chunkPos := protocol.ChunkPos{int32(blockPos[0]) >> 4, int32(blockPos[2]) >> 4}
		source := w.Chunk(chunkPos)
		if source == nil {
			x.pending.add(blockPos)
			continue
		}

		layer := (blockPos[1] >> 4) - (source.Range().Min() >> 4)
		if layer < 0 || layer >= len(source.Sub()) || source.Sub()[layer].Empty() {
			x.pending.add(blockPos)
		}

		runtimeID := df_world.BlockRuntimeID(w.Block(blockPos))
		if !x.engine.ObfuscatesBlock(runtimeID, dimension) {
			continue
		}

		x.addUpdate(updates, protocol.BlockPos{int32(blockPos[0]), int32(blockPos[1]), int32(blockPos[2])}, runtimeID)
	}
	return updates
}

func (x *chunkObfuscation) addUpdate(updates subChunkUpdates, pos protocol.BlockPos, runtimeID uint32) {
	subChunkPos := protocol.SubChunkPos{pos.X() >> 4, pos.Y() >> 4, pos.Z() >> 4}
	updates[subChunkPos] = append(updates[subChunkPos], protocol.BlockChangeEntry{BlockPos: pos, BlockRuntimeID: x.player.EncodeBlockRuntimeID(runtimeID), Flags: packet.BlockUpdateNetwork})
}

func (x *chunkObfuscation) send(updates subChunkUpdates) {
	for pos, entries := range updates {
		_ = x.player.SendPacketToClient(&packet.UpdateSubChunkBlocks{Position: protocol.BlockPos{pos[0], pos[1], pos[2]}, Blocks: entries})
	}
}
