package oconfig

type NetworkOpts struct {
	AttemptFixChunks     bool `json:"attempt_fix_chunks" comment:"If enabled, Oomph will attempt to fix chunks sent by the server to prevent invisible blocks on the client-side.\nIt is recommended to only enable this option when experiencing issues with invisible blocks, as it will cause the proxy to re-encode chunk packets sent by the server."`
	UpgradeChunksToBlobs bool `json:"upgrade_chunks_to_blobs" comment:"If enabled, Oomph will automatically upgrade chunks sent by the server to use the Minecraft sub-chunk caching system.\nThis system allows for your players to use less bandwidth and a slight increase in FPS. It is recommended to keep this option enabled."`

	GlobalMovementCutoffThreshold int `json:"global_movement_cutoff_threshold" comment:"The maximum amount of latency deviation in ticks Oomph will allow for before applying certain updates instantly without lag compensation.\nSet to -1 to disable."`
	MaxGhostBlockChain            int `json:"max_ghost_block_chain" comment:"The maximum amount of ghost blocks are allowed to be placed against each other until Oomph will discard them.\nYou may set this to -1 to disable and allow unlimited ghost blocks, although it is NOT recommended and can be abused\nby cheaters to give the illusion of flight (since other players cannot see ghost blocks they place)."`
	MaxACKTimeout                 int `json:"max_ack_timeout" comment:"The maximum amount of seconds Oomph will allow for no ACKs to be received before disconnecting the player.\nValid values are between 10 and 120 seconds."`
	MaxEntityRewind               int `json:"max_entity_rewind" comment:"The maximum amount of ticked positions Oomph should store for each entity for combat rewind and simulation.\nThis value is capped at 20 ticks (1000ms).\nThis option is not applied if Combat.FullAuthoritative is set to false. Set to -1 to disable."`
	MaxKnockbackDelay             int `json:"max_kb_delay" comment:"The maximum amount of player movements Oomph will accept before applying knockback forcefully.\nSet to -1 to disable."`
	MaxBlockUpdateDelay           int `json:"max_block_update_delay" comment:"The maximum amount of player movements Oomph will allow the client's world state to be out of sync with the server until\nforcing a block update.\nSet to -1 to disable."`
}

func Network() NetworkOpts {
	return Global.Network
}
