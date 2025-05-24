package game

const (
	DefaultJumpHeight     = float32(0.42)
	DefaultAirFriction    = float32(0.91)
	DefaultBlockFriction  = float32(0.6)
	GravityMultiplier     = float32(0.98)
	NormalGravity         = float32(0.08)
	SlowFallingGravity    = float32(0.01)
	StepHeight            = float32(0.6)
	SlideOffsetMultiplier = float32(0.4)
	SlimeBounceMultiplier = float32(-1)
	BedBounceMultiplier   = float32(-0.66)
	// This can be validated in Mob::ascendLadder()
	ClimbSpeed = float32(0.2)

	DefaultPlayerHeightOffset  = float32(1.62)
	SneakingPlayerHeightOffset = float32(1.54)

	JumpDelayTicks  = 10
	GlideBoostTicks = 20
)
