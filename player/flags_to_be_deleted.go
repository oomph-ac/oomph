package player

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

const (
	FlagTeleporting = 1 << iota
	FlagSneaking
	FlagClicking
	FlagSprinting
	FlagInVoid
	FlagInUnloadedChunk
	FlagImmobile
	FlagFlying
	FlagDead
	FlagOnGround
	FlagCollidedVertically
	FlagCollidedHorizontally
	FlagJumping
)

var InputFlagMap = map[[2]uint64]uint64{
	[2]uint64{packet.InputFlagStartSneaking, packet.InputFlagStopSneaking}:   FlagSneaking,
	[2]uint64{packet.InputFlagStartSprinting, packet.InputFlagStopSprinting}: FlagSprinting,
}
