package bedsim

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// ClientState holds non-authoritative movement data sent by the client.
type ClientState struct {
	Pos, LastPos mgl64.Vec3
	Vel, LastVel mgl64.Vec3
	Mov, LastMov mgl64.Vec3

	HorizontalCollision bool
	VerticalCollision   bool
	ToggledFly          bool
}

// MovementState holds the authoritative movement state for a single entity.
type MovementState struct {
	Client ClientState

	Pos, LastPos mgl64.Vec3
	Vel, LastVel mgl64.Vec3
	Mov, LastMov mgl64.Vec3

	Rotation, LastRotation mgl64.Vec3

	SlideOffset mgl64.Vec2
	Impulse     mgl64.Vec2
	Size        mgl64.Vec3

	SupportingBlockPos *cube.Pos

	Gravity      float64
	JumpHeight   float64
	FallDistance float64

	MovementSpeed        float64
	DefaultMovementSpeed float64
	AirSpeed             float64
	ServerUpdatedSpeed   bool

	Knockback           mgl64.Vec3
	TicksSinceKnockback uint64

	PendingTeleportPos mgl64.Vec3
	PendingTeleports   int

	TeleportPos             mgl64.Vec3
	TicksSinceTeleport      uint64
	TeleportCompletionTicks uint64
	TeleportIsSmoothed      bool

	Sprinting, PressingSprint         bool
	ServerSprint, ServerSprintApplied bool

	Sneaking, PressingSneak bool

	Jumping, PressingJump bool
	JumpDelay             uint64

	CollideX, CollideY, CollideZ bool
	OnGround                     bool

	PenetratedLastFrame, StuckInCollider bool

	Immobile bool
	NoClip   bool

	Gliding         bool
	GlideBoostTicks int64

	HasGravity bool

	Flying, MayFly, TrustFlyStatus bool
	JustDisabledFlight             bool

	AllowedInputs int64
	HasFirstInput bool

	PendingCorrections   int
	InCorrectionCooldown bool

	Ready bool
	Alive bool

	GameMode int32
}

func (s *MovementState) SetPos(newPos mgl64.Vec3) {
	s.LastPos = s.Pos
	s.Pos = newPos
}

func (s *MovementState) SetVel(newVel mgl64.Vec3) {
	s.LastVel = s.Vel
	s.Vel = newVel
}

func (s *MovementState) SetMov(newMov mgl64.Vec3) {
	s.LastMov = s.Mov
	s.Mov = newMov
}

func (s *MovementState) SetRotation(newRot mgl64.Vec3) {
	s.LastRotation = s.Rotation
	s.Rotation = newRot
}

func (s *MovementState) HasKnockback() bool {
	return s.TicksSinceKnockback == 0
}

func (s *MovementState) HasTeleport() bool {
	return s.TicksSinceTeleport <= s.TeleportCompletionTicks
}

func (s *MovementState) RemainingTeleportTicks() int {
	return int(s.TeleportCompletionTicks) - int(s.TicksSinceTeleport)
}
