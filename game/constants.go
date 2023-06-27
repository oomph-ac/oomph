package game

const (
	// DefaultJumpMotion is the default amount of motion for a jump.
	DefaultJumpMotion = float32(0.42)
	// DefaultAirFriction is the default air friction for a player
	DefaultAirFriction = float32(0.91)
	// GravityMultiplier is the multiplier for player gravity.
	GravityMultiplier = float32(0.98)
	// NormalGravity is the normal gravity for players.
	NormalGravity = float32(0.08)
	// SlowFallingGravity is the gravity for players who have the slow falling effect enabled.
	SlowFallingGravity = float32(0.01)
	// StepHeight is the maximum height a player can step up.
	StepHeight = float32(0.6)
)
