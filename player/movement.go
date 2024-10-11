package player

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// MovementComponent is an interface for which movement information is stored in the player.
// It is used for updating movement states of the player, and providing them to any other
// component that requires it.
type MovementComponent interface {
	// Pos returns the position of the movement component.
	Pos() mgl32.Vec3
	// LastPos returns the previous position of the movement component.
	LastPos() mgl32.Vec3
	// SetPos sets the position of the movement component.
	SetPos(pos mgl32.Vec3)

	// Vel returns the velocity of the movement component.
	Vel() mgl32.Vec3
	// LastVel returns the previous velocity of the movement component.
	LastVel() mgl32.Vec3
	// SetVel returns the velocity of the movement component.
	SetVel(vel mgl32.Vec3)

	// Mov returns the velocity of the movement component before friction and
	// gravity are applied to it.
	Mov() mgl32.Vec3
	// LastMov returns the previous processed velocity before friction and gravity
	// of the movement component.
	LastMov() mgl32.Vec3
	// SetMov sets the velocity of the movement component before friction and gravity.
	SetMov(mov mgl32.Vec3)

	// Rotation returns the rotation of the movement component. The X-axis contains
	// the pitch, the Y-axis contains the head-yaw, and the Z-axis contains the yaw.
	Rotation() mgl32.Vec3
	// LastRotation returns the previous rotation of the movement component.
	LastRotation() mgl32.Vec3
	// SetRotation sets the current rotation of the movement component.
	SetRotation(rot mgl32.Vec3)
	// RotationDelta returns the difference from the current and previous rotations of
	// the movement component.
	RotationDelta() mgl32.Vec3

	// Impulse returns the movement impulse of the movement component. The X-axis contains
	// the forward impulse, and the Y-axis contains the left impulse.
	Impulse() mgl32.Vec2
	// SetImpulse sets the movement impulse of the movement component.
	SetImpulse(impulse mgl32.Vec2)

	// Sprinting returns true if the movement component is sprinting.
	Sprinting() bool
	// SetSprinting sets wether or not the movement component is sprinting.
	SetSprinting(sprint bool)

	// Jumping returns true if the movement component is expecting a jump in the current frame.
	Jumping() bool
	// SetJumping sets wether or not the movement component is expecting a jump in the current frame.
	SetJumping(jumping bool)
	// PressingJump returns true if the movement component is holding down the key bound to the jump action.
	PressingJump() bool
	// SetPressingJump sets wether or not the movement component is holding down the key bound to the jump action.
	SetPressingJump(pressing bool)
	// JumpDelay returns the number of ticks until the movement component can make another jump.
	JumpDelay() uint64
	// SetJumpDelay sets the number of ticks until the movement component can make another jump.
	SetJumpDelay(ticks uint64)

	// Sneaking returns true if the movement component is currently sneaking.
	Sneaking() bool
	// SetSneaking sets wether or not the movement component is currently sneaking.
	SetSneaking(sneaking bool)
	// PressingSneak returns true if the movement component is holding down the key bound to the sneak action.
	PressingSneak() bool
	// SetPressingSneak sets if the movement component is holding down the key bound o the sneak action.
	SetPressingSneak(pressing bool)

	// PenetratedLastFrame returns true if the movement component had penetrated through a block in
	// the previous simulation frame.
	PenetratedLastFrame() bool
	// SetPenetratedLastFrame sets wether or not the movement component had penetrated through a block
	// in the previous simulation frame.
	SetPenetratedLastFrame(penetrated bool)
	// StuckInCollider returns true if the movement component is stuck in a block that does
	// not support one-way collisions.
	StuckInCollider() bool
	// SetStuckInCollider sets wether or not the movement component is stuck in a block that does
	// not support one-way collisions.
	SetStuckInCollider(stuck bool)

	// Knockback returns the knockback sent by the server to the movement component.
	Knockback() mgl32.Vec3
	// SetKnockback notifies the movement component of knockback sent by the server.
	SetKnockback(vel mgl32.Vec3)
	// HasKnockback returns true if the movement component needs knockback applied on the next simulation.
	HasKnockback() bool

	// Teleport notifies the movement component of a teleport.
	Teleport(pos mgl32.Vec3, onGround bool, smoothed bool)
	// TeleportPos returns the teleport position sent to the movement component.
	TeleportPos() mgl32.Vec3
	// HasTeleport returns true if the movement component needs a teleport applied on the next simulation.
	HasTeleport() bool

	// Size returns the width and height of the movement component in a Vec2. The X-axis
	// contains the width, and the Y-axis contains the height.
	Size() mgl32.Vec2
	// SetSize sets the size of the movement component.
	SetSize(size mgl32.Vec2)
	// BoundingBox returns the bounding box of the movement component translated to
	// it's current position.
	BoundingBox() cube.BBox

	// Gravity returns the gravity of the movement component.
	Gravity() float32
	// SetGravity sets the gravity of the movement component.
	SetGravity(gravity float32)

	// MovementSpeed returns the movement speed of the movement component.
	MovementSpeed() float32
	// SetMovementSpeed sets the movement speed of the movement component.
	SetMovementSpeed(speed float32)

	// AirSpeed returns the movement speed of the movement component while off ground.
	AirSpeed() float32
	// SetAirSpeed sets the movement speed of the movement component while off ground.
	SetAirSpeed(airSpeed float32)

	// JumpHeight returns the jump height of the movement component.
	JumpHeight() float32
	// SetJumpHeight sets the jump height of the movement component.
	SetJumpHeight(height float32)

	// XCollision returns true if the movement component is collided with a block
	// on the x-axis.
	XCollision() bool
	// YCollision returns true if the movement component is collided with a block
	// on the y-axis.
	YCollision() bool
	// ZCollision returns true if the movement component is collided with a block
	// on the z-axis.
	ZCollision() bool
	// SetCollisions sets wether or not the movement component is colliding with a block
	// on the x, y, or z axies.
	SetCollisions(x, y, z bool)

	// OnGround returns true if the movement component is on the ground.
	OnGround() bool
	// SetOnGround sets wether or not the movement component is on the ground.
	SetOnGround(onGround bool)

	// NoClientPredictions returns true if the movement component does not need their movement simulated.
	NoClientPredictions() bool
	// SetNoClientPredictions sets wether or not the movement component needxs their movement simulated.
	SetNoClientPredictions(noClip bool)

	// CanSimulate returns true if the movement component can be simulated by the server for the current frame.
	CanSimulate() bool
	// SetCanSimulate sets wether or not the movement component can be simulated by the server for the current frame.
	SetCanSimulate(simulate bool)

	// Update updates the states of the movement component from the given input.
	Update(input *packet.PlayerAuthInput)
	// Simulate does any simulations needed by the movement component.
	Simulate()

	// Validate is a function that returns true if this movement component has a position within
	// the given threshold of the other movement component.
	Validate(threshold float32, other MovementComponent) bool
}
