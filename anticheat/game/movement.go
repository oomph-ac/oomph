package game

const (
	DefaultJumpHeight    = float32(0.42)
	NormalGravity        = float32(0.08)
	SlowFallingGravity   = float32(0.01)
	MaxConsumingImpulse  = float32(0.1225)
	MaxSneakImpulse      = float32(0.3)
	MaxNormalizedImpulse = float32(0.70710678118) // 1/sqrt(2)

	DefaultPlayerHeightOffset  = float32(1.62)
	SneakingPlayerHeightOffset = float32(1.27)

	GlideBoostTicks = 20
)
