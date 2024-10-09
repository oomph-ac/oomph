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
	// CanSimulate returns true if the movement component can be simulated by the server for the current frame.
	CanSimulate() bool
	// SetCanSimulate sets wether or not the movement component can be simulated by the server for the current frame.
	SetCanSimulate(simulate bool)

	// Pos returns the position of the movement component.
	Pos() mgl32.Vec3
	// LastPos returns the previous position of the movement component.
	LastPos() mgl32.Vec3
	// SetPos sets the position of the movement component.
	SetPos(pos mgl32.Vec3)
	// PosDelta returns the difference between the current and previous position
	// of the movement component.
	PosDelta() mgl32.Vec3

	// Vel sets the velocity of the movement component.
	Vel() mgl32.Vec3
	// LastVel returns the previous velocity of the movement component.
	LastVel() mgl32.Vec3
	// SetVel returns the velocity of the movement component.
	SetVel(vel mgl32.Vec3)

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
	JumpDelay() int64
	// SetJumpDelay sets the number of ticks until the movement component can make another jump.
	SetJumpDelay(ticks int64)

	// Sneaking returns true if the movement component is currently sneaking.
	Sneaking() bool
	// SetSneaking sets wether or not the movement component is currently sneaking.
	SetSneaking(bool)
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

	// NoClip returns true if the movement component is able to no-clip.
	NoClip() bool
	// SetNoClip sets wether or not the movement component is able to no-clip.
	SetNoClip(noClip bool)

	// Knockback returns the knockback sent by the server to the movement component.
	Knockback() mgl32.Vec3
	// SetKnockback notifies the movement component of knockback sent by the server.
	SetKnockback(vel mgl32.Vec3)

	// Teleport notifies the movement component of a teleport.
	Teleport(pos mgl32.Vec3, onGround bool, smoothed bool)
	// TeleportPos returns the teleport position sent to the movement component.
	TeleportPos() mgl32.Vec3

	// Size returns the width and height of the movement component in a Vec2. The X-axis
	// contains the width, and the Y-axis contains the height.
	Size() mgl32.Vec2
	// SetSize sets the size of the movement component.
	SetSize(size mgl32.Vec2)
	// BoundingBox returns the bounding box of the movement component translated to
	// it's current position.
	BoundingBox() cube.BBox

	// Update is a function that updates the states of the movement component based on the
	// client's input for the current simulation frame.
	Update(input *packet.PlayerAuthInput)
}
