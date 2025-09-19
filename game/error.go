package game

const (
	ErrorNotReady              = "Error: Client did not initialize correctly."
	ErrorNetworkTimeout        = "Error: Network timed out."
	ErrorChunkCacheUnsupported = "Error: Chunk cache is not supported."

	ErrorInvalidInventorySlot = "Error: Invalid inventory slot selected."
	ErrorInvalidBlockFace     = "Error: Invalid block face selected."

	ErrorInternalDecodeChunk                       = "Unable to decode chunk: %v"
	ErrorInternalDuplicateACK                      = "Error: Duplicated ACKs."
	ErrorInternalACKIsNull                         = "Error: Attempt to send client null ACK."
	ErrorInternalUnexpectedNullInput               = "Error: Combat handler encountered null input."
	ErrorInternalMissingMovementComponent          = "Error: Movement component required to simulate movement."
	ErrorInternalInvalidPacketForMovementComponent = "Error: Movement component cannot process %T."
)
