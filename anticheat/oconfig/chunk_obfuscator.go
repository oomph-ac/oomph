package oconfig

type ObfuscationMode string

const (
	ObfuscationModeHide    ObfuscationMode = "hide"
	ObfuscationModeRandom  ObfuscationMode = "random"
	ObfuscationModeLayered ObfuscationMode = "layered"
)

type ChunkObfuscatorOpts struct {
	Enabled     bool                          `json:"enabled" comment:"Whether Oomph should obfuscate underground blocks before sending chunks to players."`
	BlockRadius int                           `json:"block_radius" comment:"The radius around a block update where hidden blocks are shown."`
	Dimensions  ChunkObfuscatorDimensionsOpts `json:"dimensions"`
}

type ChunkObfuscatorDimensionsOpts struct {
	Overworld ChunkObfuscatorDimensionOpts `json:"overworld"`
	Nether    ChunkObfuscatorDimensionOpts `json:"nether"`
}

type ChunkObfuscatorDimensionOpts struct {
	Enabled              bool            `json:"enabled" comment:"Whether chunk obfuscation is enabled in this dimension."`
	Mode                 ObfuscationMode `json:"mode" comment:"The obfuscation mode. Allowed values are hide, random, and layered."`
	MaxY                 int             `json:"max_y" comment:"The highest block Y coordinate that Oomph obfuscates."`
	HiddenBlocks         []string        `json:"hidden_blocks" comment:"Blocks hidden by hide mode and used as decoys by random and layered modes."`
	TerrainBlocks        []string        `json:"terrain_blocks" comment:"Solid terrain blocks eligible for random and layered obfuscation."`
	ReplacementBlock     string          `json:"replacement_block" comment:"The block used by hide mode at Y 0 and above."`
	DeepReplacementBlock string          `json:"deep_replacement_block" comment:"The block used by hide mode below Y 0."`
}

func ChunkObfuscator() ChunkObfuscatorOpts {
	return Global.ChunkObfuscator
}
