package world

// ChunkSource is an interface that returns block information like a regular chunk.
type ChunkSource interface {
	// Block returns the block at the given position and layer of the chunk source.
	Block(x uint8, y int16, z uint8, layer uint8) (rid uint32)
}
