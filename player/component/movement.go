package component

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/assert"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type AuthoritativeMovementComponent struct {
	mPlayer  *player.Player
	fallback *AuthoritativeMovementComponent

	pos, lastPos           mgl32.Vec3
	vel, lastVel           mgl32.Vec3
	mov, lastMov           mgl32.Vec3
	rotation, lastRotation mgl32.Vec3

	impulse mgl32.Vec2
	size    mgl32.Vec2

	gravity                 float32
	jumpHeight              float32
	movementSpeed, airSpeed float32

	knockback    mgl32.Vec3
	ticksSinceKb uint64

	teleportPos        mgl32.Vec3
	ticksSinceTeleport uint64
	teleportIsSmoothed bool

	sprinting               bool
	sneaking, pressingSneak bool

	jumping, pressingJump bool
	jumpDelay             uint64

	collideX, collideY, collideZ bool
	onGround                     bool

	penetratedLastFrame, stuckInCollider bool
	clientHasNoPredictions, canSimulate  bool
}

// Pos returns the position of the movement component.
func (mc *AuthoritativeMovementComponent) Pos() mgl32.Vec3 {
	return mc.pos
}

// LastPos returns the previous position of the movement component.
func (mc *AuthoritativeMovementComponent) LastPos() mgl32.Vec3 {
	return mc.lastPos
}

// SetPos sets the position of the movement component.
func (mc *AuthoritativeMovementComponent) SetPos(newPos mgl32.Vec3) {
	mc.lastPos = mc.pos
	mc.pos = newPos
}

// Vel returns the velocity of the movement component.
func (mc *AuthoritativeMovementComponent) Vel() mgl32.Vec3 {
	return mc.vel
}

// LastVel returns the previous velocity of the movement component.
func (mc *AuthoritativeMovementComponent) LastVel() mgl32.Vec3 {
	return mc.lastVel
}

// SetVel returns the velocity of the movement component.
func (mc *AuthoritativeMovementComponent) SetVel(newVel mgl32.Vec3) {
	mc.lastVel = mc.vel
	mc.vel = newVel
}

// Mov returns the velocity of the movement component before friction and
// gravity are applied to it.
func (mc *AuthoritativeMovementComponent) Mov() mgl32.Vec3 {
	return mc.mov
}

// LastMov returns the previous processed velocity before friction and gravity
// of the movement component.
func (mc *AuthoritativeMovementComponent) LastMov() mgl32.Vec3 {
	return mc.lastMov
}

// SetMov sets the velocity of the movement component before friction and gravity.
func (mc *AuthoritativeMovementComponent) SetMov(newMov mgl32.Vec3) {
	mc.lastMov = mc.mov
	mc.mov = newMov
}

// Rotation returns the rotation of the movement component. The X-axis contains
// the pitch, the Y-axis contains the head-yaw, and the Z-axis contains the yaw.
func (mc *AuthoritativeMovementComponent) Rotation() mgl32.Vec3 {
	return mc.rotation
}

// LastRotation returns the previous rotation of the movement component.
func (mc *AuthoritativeMovementComponent) LastRotation() mgl32.Vec3 {
	return mc.lastRotation
}

// SetRotation sets the current rotation of the movement component.
func (mc *AuthoritativeMovementComponent) SetRotation(newRotation mgl32.Vec3) {
	mc.lastRotation = mc.rotation
	mc.rotation = newRotation
}

// RotationDelta returns the difference from the current and previous rotations of
// the movement component.
func (mc *AuthoritativeMovementComponent) RotationDelta() mgl32.Vec3 {
	return mc.rotation.Sub(mc.lastRotation)
}

// Impulse returns the movement impulse of the movement component. The X-axis contains
// the forward impulse, and the Y-axis contains the left impulse.
func (mc *AuthoritativeMovementComponent) Impulse() mgl32.Vec2 {
	return mc.impulse
}

// SetImpulse sets the movement impulse of the movement component.
func (mc *AuthoritativeMovementComponent) SetImpulse(newImpulse mgl32.Vec2) {
	mc.impulse = newImpulse
}

// Sprinting returns true if the movement component is sprinting.
func (mc *AuthoritativeMovementComponent) Sprinting() bool {
	return mc.sprinting
}

// SetSprinting sets wether or not the movement component is sprinting.
func (mc *AuthoritativeMovementComponent) SetSprinting(sprinting bool) {
	mc.sprinting = sprinting
}

// Jumping returns true if the movement component is expecting a jump in the current frame.
func (mc *AuthoritativeMovementComponent) Jumping() bool {
	return mc.jumping
}

// SetJumping sets wether or not the movement component is expecting a jump in the current frame.
func (mc *AuthoritativeMovementComponent) SetJumping(jumping bool) {
	mc.jumping = jumping
}

// PressingJump returns true if the movement component is holding down the key bound to the jump action.
func (mc *AuthoritativeMovementComponent) PressingJump() bool {
	return mc.pressingJump
}

// SetPressingJump sets wether or not the movement component is holding down the key bound to the jump action.
func (mc *AuthoritativeMovementComponent) SetPressingJump(pressing bool) {
	mc.pressingJump = pressing
}

// JumpDelay returns the number of ticks until the movement component can make another jump.
func (mc *AuthoritativeMovementComponent) JumpDelay() uint64 {
	return mc.jumpDelay
}

// SetJumpDelay sets the number of ticks until the movement component can make another jump.
func (mc *AuthoritativeMovementComponent) SetJumpDelay(ticks uint64) {
	mc.jumpDelay = ticks
}

// Sneaking returns true if the movement component is currently sneaking.
func (mc *AuthoritativeMovementComponent) Sneaking() bool {
	return mc.sneaking
}

// SetSneaking sets wether or not the movement component is currently sneaking.
func (mc *AuthoritativeMovementComponent) SetSneaking(sneaking bool) {
	mc.sneaking = sneaking
}

// PressingSneak returns true if the movement component is holding down the key bound to the sneak action.
func (mc *AuthoritativeMovementComponent) PressingSneak() bool {
	return mc.pressingSneak
}

// SetPressingSneak sets if the movement component is holding down the key bound o the sneak action.
func (mc *AuthoritativeMovementComponent) SetPressingSneak(pressing bool) {
	mc.pressingSneak = pressing
}

// PenetratedLastFrame returns true if the movement component had penetrated through a block in
// the previous simulation frame.
func (mc *AuthoritativeMovementComponent) PenetratedLastFrame() bool {
	return mc.penetratedLastFrame
}

// SetPenetratedLastFrame sets wether or not the movement component had penetrated through a block
// in the previous simulation frame.
func (mc *AuthoritativeMovementComponent) SetPenetratedLastFrame(penetrated bool) {
	mc.penetratedLastFrame = penetrated
}

// StuckInCollider returns true if the movement component is stuck in a block that does
// not support one-way collisions.
func (mc *AuthoritativeMovementComponent) StuckInCollider() bool {
	return mc.stuckInCollider
}

// SetStuckInCollider sets wether or not the movement component is stuck in a block that does
// not support one-way collisions.
func (mc *AuthoritativeMovementComponent) SetStuckInCollider(stuck bool) {
	mc.stuckInCollider = stuck
}

// Knockback returns the knockback sent by the server to the movement component.
func (mc *AuthoritativeMovementComponent) Knockback() mgl32.Vec3 {
	return mc.knockback
}

// SetKnockback notifies the movement component of knockback sent by the server.
func (mc *AuthoritativeMovementComponent) SetKnockback(newKnockback mgl32.Vec3) {
	mc.knockback = newKnockback
	mc.ticksSinceKb = 0
}

// HasKnockback returns true if the movement component needs knockback applied on the next simulation.
func (mc *AuthoritativeMovementComponent) HasKnockback() bool {
	return mc.ticksSinceKb == 0
}

// Teleport notifies the movement component of a teleport.
func (mc *AuthoritativeMovementComponent) Teleport(pos mgl32.Vec3, onGround bool, smoothed bool) {
	mc.teleportPos = pos
	mc.onGround = onGround
	mc.teleportIsSmoothed = smoothed
	mc.ticksSinceTeleport = 0
}

// TeleportPos returns the teleport position sent to the movement component.
func (mc *AuthoritativeMovementComponent) TeleportPos() mgl32.Vec3 {
	return mc.teleportPos
}

// HasTeleport returns true if the movement component needs a teleport applied on the next simulation.
func (mc *AuthoritativeMovementComponent) HasTeleport() bool {
	return mc.ticksSinceTeleport == 0
}

// Size returns the width and height of the movement component in a Vec2. The X-axis
// contains the width, and the Y-axis contains the height.
func (mc *AuthoritativeMovementComponent) Size() mgl32.Vec2 {
	return mc.size
}

// SetSize sets the size of the movement component.
func (mc *AuthoritativeMovementComponent) SetSize(newSize mgl32.Vec2) {
	mc.size = newSize
}

// BoundingBox returns the bounding box of the movement component translated to
// it's current position.
func (mc *AuthoritativeMovementComponent) BoundingBox() cube.BBox {
	width := mc.size[0] / 2
	return cube.Box(
		mc.pos[0]-width,
		mc.pos[1],
		mc.pos[2]-width,
		mc.pos[0]+width,
		mc.pos[1]+mc.size[1],
		mc.pos[2]+width,
	).GrowVec3(mgl32.Vec3{-0.001, 0, -0.001})
}

// Gravity returns the gravity of the movement component.
func (mc *AuthoritativeMovementComponent) Gravity() float32 {
	return mc.gravity
}

// SetGravity sets the gravity of the movement component.
func (mc *AuthoritativeMovementComponent) SetGravity(newGravity float32) {
	mc.gravity = newGravity
}

// JumpHeight returns the jump height of the movement component.
func (mc *AuthoritativeMovementComponent) JumpHeight() float32 {
	return mc.jumpHeight
}

// SetJumpHeight sets the jump height of the movement component.
func (mc *AuthoritativeMovementComponent) SetJumpHeight(jumpHeight float32) {
	mc.jumpHeight = jumpHeight
}

// MovementSpeed returns the movement speed of the movement component.
func (mc *AuthoritativeMovementComponent) MovementSpeed() float32 {
	return mc.movementSpeed
}

// SetMovementSpeed sets the movement speed of the movement component.
func (mc *AuthoritativeMovementComponent) SetMovementSpeed(newSpeed float32) {
	mc.movementSpeed = newSpeed
}

// AirSpeed returns the movement speed of the movement component while off ground.
func (mc *AuthoritativeMovementComponent) AirSpeed() float32 {
	return mc.airSpeed
}

// SetAirSpeed sets the movement speed of the movement component while off ground.
func (mc *AuthoritativeMovementComponent) SetAirSpeed(newSpeed float32) {
	mc.airSpeed = newSpeed
}

// XCollision returns true if the movement component is collided with a block
// on the x-axis.
func (mc *AuthoritativeMovementComponent) XCollision() bool {
	return mc.collideX
}

// YCollision returns true if the movement component is collided with a block
// on the y-axis.
func (mc *AuthoritativeMovementComponent) YCollision() bool {
	return mc.collideY
}

// ZCollision returns true if the movement component is collided with a block
// on the z-axis.
func (mc *AuthoritativeMovementComponent) ZCollision() bool {
	return mc.collideZ
}

// SetCollisions sets wether or not the movement component is colliding with a block
// on the x, y, or z axies.
func (mc *AuthoritativeMovementComponent) SetCollisions(xCollision, yCollision, zCollision bool) {
	mc.collideX = xCollision
	mc.collideY = yCollision
	mc.collideZ = zCollision
}

// OnGround returns true if the movement component is on the ground.
func (mc *AuthoritativeMovementComponent) OnGround() bool {
	return mc.onGround
}

// SetOnGround sets wether or not the movement component is on the ground.
func (mc *AuthoritativeMovementComponent) SetOnGround(onGround bool) {
	mc.onGround = onGround
}

// NoClientPredictions returns true if the movement component does not need their movement simulated.
func (mc *AuthoritativeMovementComponent) NoClientPredictions() bool {
	return mc.clientHasNoPredictions
}

// SetNoClientPredictions sets wether or not the movement component needxs their movement simulated.
func (mc *AuthoritativeMovementComponent) SetNoClientPredictions(noPredictions bool) {
	mc.clientHasNoPredictions = noPredictions
}

// CanSimulate returns true if the movement component can be simulated by the server for the current frame.
func (mc *AuthoritativeMovementComponent) CanSimulate() bool {
	return mc.canSimulate
}

// SetCanSimulate sets wether or not the movement component can be simulated by the server for the current frame.
func (mc *AuthoritativeMovementComponent) SetCanSimulate(canSim bool) {
	mc.canSimulate = canSim
}

// Update updates the states of the movement component from the given input.
func (mc *AuthoritativeMovementComponent) Update(input *packet.PlayerAuthInput) {
	assert.IsTrue(mc.mPlayer != nil, "parent player is null")
	assert.IsTrue(input != nil, "given player input is nil")

	mc.impulse = input.MoveVector

	startFlag, stopFlag := utils.HasFlag(input.InputData, packet.InputFlagStartSprinting), utils.HasFlag(input.InputData, packet.InputFlagStopSprinting)
	if (startFlag && stopFlag) || stopFlag {
		mc.sprinting = false
	} else if startFlag {
		mc.sprinting = true
	}

	mc.jumping = utils.HasFlag(input.InputData, packet.InputFlagStartJumping)
	mc.pressingJump = utils.HasFlag(input.InputData, packet.InputFlagJumping)
	mc.jumpHeight = game.DefaultJumpHeight

	// TODO: Effects component.
}

// Simulate does any simulations needed by the movement component.
func (mc *AuthoritativeMovementComponent) Simulate() {
	if !mc.canSimulate {
		return
	}
}

// Validate is a function that returns true if this movement component has a position within
// the given threshold of the other movement component.
func (mc *AuthoritativeMovementComponent) Validate(threshold float32, other player.MovementComponent) bool {
	if !mc.canSimulate {
		return true
	}
	return other.Pos().Sub(mc.pos).Len() <= threshold
}
