package component

import (
	"fmt"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/oomph-ac/oomph/player/simulation"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var playerHeightOffset = mgl32.Vec3{0, game.DefaultPlayerHeightOffset}

// NonAuthoritativeMovementInfo represents the velocity and position that the player has sent to the server but has not validated.
type NonAuthoritativeMovement struct {
	pos, lastPos mgl32.Vec3
	vel, lastVel mgl32.Vec3
	mov, lastMov mgl32.Vec3

	toggledFly bool

	horizontalCollision bool
	verticalCollision   bool
}

func (m *NonAuthoritativeMovement) Pos() mgl32.Vec3 {
	return m.pos
}

func (m *NonAuthoritativeMovement) LastPos() mgl32.Vec3 {
	return m.lastPos
}

func (m *NonAuthoritativeMovement) Vel() mgl32.Vec3 {
	return m.vel
}

func (m *NonAuthoritativeMovement) LastVel() mgl32.Vec3 {
	return m.lastVel
}

func (m *NonAuthoritativeMovement) Mov() mgl32.Vec3 {
	return m.mov
}

func (m *NonAuthoritativeMovement) LastMov() mgl32.Vec3 {
	return m.lastMov
}

// ToggledFly returns whether the movement component has attempted to trigger a fly action.
func (m *NonAuthoritativeMovement) ToggledFly() bool {
	return m.toggledFly
}

// SetToggledFly sets whether the movement component has attempted to trigger a fly action.
func (m *NonAuthoritativeMovement) SetToggledFly(toggled bool) {
	m.toggledFly = toggled
}

func (m *NonAuthoritativeMovement) HorizontalCollision() bool {
	return m.horizontalCollision
}

func (m *NonAuthoritativeMovement) VerticalCollision() bool {
	return m.verticalCollision
}

type AuthoritativeMovementComponent struct {
	mPlayer          *player.Player
	nonAuthoritative *NonAuthoritativeMovement

	pos, lastPos           mgl32.Vec3
	vel, lastVel           mgl32.Vec3
	mov, lastMov           mgl32.Vec3
	rotation, lastRotation mgl32.Vec3

	slideOffset mgl32.Vec2
	impulse     mgl32.Vec2
	size        mgl32.Vec3

	supportingBlockPos *cube.Pos

	gravity      float32
	jumpHeight   float32
	fallDistance float32

	movementSpeed        float32
	defaultMovementSpeed float32
	airSpeed             float32
	serverUpdatedSpeed   bool

	knockback    mgl32.Vec3
	ticksSinceKb uint64

	pendingTeleportPos mgl32.Vec3
	pendingTeleports   int

	teleportPos             mgl32.Vec3
	ticksSinceTeleport      uint64
	teleportCompletionTicks uint64
	teleportIsSmoothed      bool

	sprinting, pressingSprint         bool
	serverSprint, serverSprintApplied bool

	sneaking, pressingSneak bool

	jumping, pressingJump bool
	jumpDelay             uint64

	collideX, collideY, collideZ bool
	onGround                     bool

	penetratedLastFrame, stuckInCollider bool

	immobile bool
	noClip   bool

	gliding         bool
	glideBoostTicks int64

	flying, mayFly, trustFlyStatus bool
	justDisabledFlight             bool

	allowedInputs int64
	hasFirstInput bool

	pendingCorrections   int
	inCorrectionCooldown bool
}

func NewAuthoritativeMovementComponent(p *player.Player) *AuthoritativeMovementComponent {
	return &AuthoritativeMovementComponent{
		mPlayer:              p,
		nonAuthoritative:     &NonAuthoritativeMovement{},
		defaultMovementSpeed: 0.1,
		airSpeed:             0.02,
	}
}

// InputAcceptable returns true if the input is within the rate-limit Oomph has imposed for the player.
func (mc *AuthoritativeMovementComponent) InputAcceptable() bool {
	if !mc.hasFirstInput {
		mc.hasFirstInput = true
	}

	if mc.allowedInputs <= 0 {
		mc.mPlayer.Dbg.Notify(player.DebugModeTimer, true, "no allowed inputs remaining (cT=%d sT=%d)", mc.mPlayer.SimulationFrame, mc.mPlayer.ServerTick)
		return false
	}
	mc.mPlayer.Dbg.Notify(player.DebugModeTimer, true, "allowed inputs remaining: %d (cT=%d sT=%d)", mc.allowedInputs, mc.mPlayer.SimulationFrame, mc.mPlayer.ServerTick)
	mc.allowedInputs--
	return true
}

func (mc *AuthoritativeMovementComponent) Tick(elapsedTicks int64) {
	if !mc.hasFirstInput {
		mc.allowedInputs = 65535
		return
	}

	latencyAllowance := mc.mPlayer.ServerTick - mc.mPlayer.ClientTick
	if latencyAllowance < 0 {
		latencyAllowance = 0
	}
	latencyAllowance += elapsedTicks
	defaultAllowance := mc.allowedInputs
	if defaultAllowance < 0 {
		defaultAllowance = 0
	}
	defaultAllowance += elapsedTicks

	if latencyAllowance < defaultAllowance {
		mc.allowedInputs = latencyAllowance
	} else {
		mc.allowedInputs = defaultAllowance
	}

	// We must always accept one player input for every server tick.
	if mc.allowedInputs < 1 {
		mc.allowedInputs = 1
	}
}

// Client returns the non-authoritative movement data sent to us from the client.
func (mc *AuthoritativeMovementComponent) Client() player.NonAuthoritativeMovementInfo {
	return mc.nonAuthoritative
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

// SlideOffset returns the slide offset of the movement component.
func (mc *AuthoritativeMovementComponent) SlideOffset() mgl32.Vec2 {
	return mc.slideOffset
}

// SetSlideOffset sets the slide offset of the movement component.
func (mc *AuthoritativeMovementComponent) SetSlideOffset(slideOffset mgl32.Vec2) {
	mc.slideOffset = slideOffset
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

// RotationDelta returns the difference from the current and previous rotations of the movement component.
func (mc *AuthoritativeMovementComponent) RotationDelta() mgl32.Vec3 {
	return mc.rotation.Sub(mc.lastRotation)
}

// SupportingBlockPos returns the position of the block that the player is standing on/supported by.
func (mc *AuthoritativeMovementComponent) SupportingBlockPos() *cube.Pos {
	return mc.supportingBlockPos
}

// SetSupportingBlockPos sets the position of the block that the player is standing on/supported by.
func (mc *AuthoritativeMovementComponent) SetSupportingBlockPos(pos *cube.Pos) {
	mc.supportingBlockPos = pos
}

// Impulse returns the movement impulse of the movement component. The X-axis contains the forward impulse, and the Y-axis contains the left impulse.
func (mc *AuthoritativeMovementComponent) Impulse() mgl32.Vec2 {
	return mc.impulse
}

// Sprinting returns true if the movement component is sprinting.
func (mc *AuthoritativeMovementComponent) Sprinting() bool {
	return mc.sprinting
}

// SetSprinting sets whether the movement component is sprinting.
func (mc *AuthoritativeMovementComponent) SetSprinting(sprinting bool) {
	mc.sprinting = sprinting
}

// ServerSprint returns whether the movement component is sprinting according to the server.
func (mc *AuthoritativeMovementComponent) ServerSprint() bool {
	return mc.serverSprint
}

// SetServerSprint sets whether the movement component is sprinting according to the server.
func (mc *AuthoritativeMovementComponent) SetServerSprint(sprinting bool) {
	mc.serverSprintApplied = false
	mc.serverSprint = sprinting
}

// PressingSprint returns whether the movement component is holding down the key bound to the sprint action.
func (mc *AuthoritativeMovementComponent) PressingSprint() bool {
	return mc.pressingSprint
}

// Jumping returns true if the movement component is expecting a jump in the current frame.
func (mc *AuthoritativeMovementComponent) Jumping() bool {
	return mc.jumping
}

// PressingJump returns true if the movement component is holding down the key bound to the jump action.
func (mc *AuthoritativeMovementComponent) PressingJump() bool {
	return mc.pressingJump
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

// SetPenetratedLastFrame sets whether the movement component had penetrated through a block
// in the previous simulation frame.
func (mc *AuthoritativeMovementComponent) SetPenetratedLastFrame(penetrated bool) {
	mc.penetratedLastFrame = penetrated
}

// StuckInCollider returns true if the movement component is stuck in a block that does
// not support one-way collisions.
func (mc *AuthoritativeMovementComponent) StuckInCollider() bool {
	return mc.stuckInCollider
}

// SetStuckInCollider sets whether the movement component is stuck in a block that does
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

	if smoothed {
		mc.teleportCompletionTicks = 2
	} else {
		mc.teleportCompletionTicks = 0
	}
}

// TeleportPos returns the teleport position sent to the movement component.
func (mc *AuthoritativeMovementComponent) TeleportPos() mgl32.Vec3 {
	return mc.teleportPos
}

// HasTeleport returns true if the movement component needs a teleport applied on the next simulation.
func (mc *AuthoritativeMovementComponent) HasTeleport() bool {
	return mc.ticksSinceTeleport <= mc.teleportCompletionTicks
}

// TeleportSmoothed returns true if the movement component has a teleport that needs to be smoothed out.
func (mc *AuthoritativeMovementComponent) TeleportSmoothed() bool {
	return mc.teleportIsSmoothed
}

func (mc *AuthoritativeMovementComponent) SetPendingTeleportPos(pos mgl32.Vec3) {
	mc.pendingTeleportPos = pos
}

func (mc *AuthoritativeMovementComponent) PendingTeleportPos() mgl32.Vec3 {
	return mc.pendingTeleportPos
}

func (mc *AuthoritativeMovementComponent) AddPendingTeleport() {
	mc.pendingTeleports++
}

func (mc *AuthoritativeMovementComponent) RemovePendingTeleport() {
	mc.pendingTeleports--
}

func (mc *AuthoritativeMovementComponent) PendingTeleports() int {
	return mc.pendingTeleports
}

// RemainingTeleportTicks returns the amount of ticks the teleport still needs to be completed.
func (mc *AuthoritativeMovementComponent) RemainingTeleportTicks() int {
	return int(mc.teleportCompletionTicks) - int(mc.ticksSinceTeleport)
}

// TicksSinceTeleport returns the amount of ticks since the last teleport was applied.
func (mc *AuthoritativeMovementComponent) TicksSinceTeleport() uint64 {
	return mc.ticksSinceTeleport
}

// Size returns the width and height of the movement component in a Vec2. The X-axis
// contains the width, and the Y-axis contains the height.
func (mc *AuthoritativeMovementComponent) Size() mgl32.Vec3 {
	return mc.size
}

// SetSize sets the size of the movement component.
func (mc *AuthoritativeMovementComponent) SetSize(newSize mgl32.Vec3) {
	mc.size = newSize
}

// BoundingBox returns the bounding box of the movement component translated to it's current position.
func (mc *AuthoritativeMovementComponent) BoundingBox() cube.BBox {
	scale := mc.size[2]
	width := (mc.size[0] * 0.5) * scale
	height := mc.size[1] * scale
	var yOffset float32
	if mc.mPlayer.VersionInRange(-1, player.GameVersion1_20_60) {
		yOffset = mc.slideOffset.Y()
	}

	return cube.Box(
		mc.pos[0]-width,
		(mc.pos[1] + yOffset),
		mc.pos[2]-width,
		mc.pos[0]+width,
		mc.pos[1]+height+yOffset,
		mc.pos[2]+width,
	).GrowVec3(mgl32.Vec3{-1e-3, 0, -1e-3})
}

// ClientBoundingBox returns the bounding box of the movement component translated to the client's position.
func (mc *AuthoritativeMovementComponent) ClientBoundingBox() cube.BBox {
	width := mc.size[0] / 2
	var yOffset float32
	if mc.mPlayer.VersionInRange(-1, player.GameVersion1_20_60) {
		yOffset = mc.slideOffset.Y()
	}

	return cube.Box(
		mc.nonAuthoritative.pos[0]-width,
		mc.nonAuthoritative.pos[1]+yOffset,
		mc.nonAuthoritative.pos[2]-width,
		mc.nonAuthoritative.pos[0]+width,
		mc.nonAuthoritative.pos[1]+mc.size[1]+yOffset,
		mc.nonAuthoritative.pos[2]+width,
	).GrowVec3(mgl32.Vec3{-1e-4, 0, -1e-4})
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

// FallDistance returns the fall distance of the movement component.
func (mc *AuthoritativeMovementComponent) FallDistance() float32 {
	return mc.fallDistance
}

// SetFallDistance sets the fall distance of the movement component.
func (mc *AuthoritativeMovementComponent) SetFallDistance(fallDistance float32) {
	mc.fallDistance = fallDistance
}

// MovementSpeed returns the movement speed of the movement component.
func (mc *AuthoritativeMovementComponent) MovementSpeed() float32 {
	return mc.movementSpeed
}

// SetMovementSpeed sets the movement speed of the movement component.
func (mc *AuthoritativeMovementComponent) SetMovementSpeed(newSpeed float32) {
	mc.movementSpeed = newSpeed
	mc.serverUpdatedSpeed = true
}

// DefaultMovementSpeed return the movement speed the client should default to.
func (mc *AuthoritativeMovementComponent) DefaultMovementSpeed() float32 {
	return mc.defaultMovementSpeed
}

// SetDefaultMovementSpeed sets the movement speed the client should default to.
func (mc *AuthoritativeMovementComponent) SetDefaultMovementSpeed(speed float32) {
	mc.defaultMovementSpeed = speed
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

// SetCollisions sets whether the movement component is colliding with a block
// on the x, y, or z axes.
func (mc *AuthoritativeMovementComponent) SetCollisions(xCollision, yCollision, zCollision bool) {
	mc.collideX = xCollision
	mc.collideY = yCollision
	mc.collideZ = zCollision
}

// OnGround returns true if the movement component is on the ground.
func (mc *AuthoritativeMovementComponent) OnGround() bool {
	return mc.onGround
}

// SetOnGround sets whether the movement component is on the ground.
func (mc *AuthoritativeMovementComponent) SetOnGround(onGround bool) {
	mc.onGround = onGround
}

// Immobile returns true if the movement component is immobile.
func (mc *AuthoritativeMovementComponent) Immobile() bool {
	return mc.immobile
}

// SetImmobile sets whether the movement component is immobile.
func (mc *AuthoritativeMovementComponent) SetImmobile(immobile bool) {
	mc.immobile = immobile
}

// NoClip returns true if the movement component has no collisions.
func (mc *AuthoritativeMovementComponent) NoClip() bool {
	return mc.noClip
}

// SetNoClip sets whether the movement component has collisions.
func (mc *AuthoritativeMovementComponent) SetNoClip(noClip bool) {
	mc.noClip = noClip
}

// Gliding returns if the movement component is gliding.
func (mc *AuthoritativeMovementComponent) Gliding() bool {
	return mc.gliding
}

// SetGliding sets whether the movement component is gliding.
func (mc *AuthoritativeMovementComponent) SetGliding(gliding bool) {
	mc.gliding = gliding
}

// GlideBoost returns the amount of ticks the movement component has a gliding boost for.
func (mc *AuthoritativeMovementComponent) GlideBoost() int64 {
	return mc.glideBoostTicks
}

// SetGlideBoost sets the amount of ticks the movement component should apply a gliding boost for.
func (mc *AuthoritativeMovementComponent) SetGlideBoost(boostTicks int64) {
	mc.glideBoostTicks = boostTicks
}

// Flying returns true if the movement component is flying.
func (mc *AuthoritativeMovementComponent) Flying() bool {
	return mc.flying
}

// SetFlying sets if the movement component is flying.
func (mc *AuthoritativeMovementComponent) SetFlying(fly bool) {
	mc.flying = fly
}

// MayFly returns true if the movement component has the permission to fly.
func (mc *AuthoritativeMovementComponent) MayFly() bool {
	return mc.mayFly
}

// SetMayFly sets whether the movement component has the permission to fly.
func (mc *AuthoritativeMovementComponent) SetMayFly(mayFly bool) {
	mc.mayFly = mayFly
}

// TrustFlyStatus returns whether the movement component can trust the fly status sent by the client.
func (mc *AuthoritativeMovementComponent) TrustFlyStatus() bool {
	return mc.trustFlyStatus
}

// SetTrustFlyStatus sets whether the movement component can trust the fly status sent by the client.
func (mc *AuthoritativeMovementComponent) SetTrustFlyStatus(trust bool) {
	mc.trustFlyStatus = trust
}

// JustDisabledFlight returns true if the movement component just disabled flight.
func (mc *AuthoritativeMovementComponent) JustDisabledFlight() bool {
	return mc.justDisabledFlight
}

// Update updates the states of the movement component from the given input.
func (mc *AuthoritativeMovementComponent) Update(pk *packet.PlayerAuthInput) {
	//assert.IsTrue(mc.mPlayer != nil, "parent player is null")
	//assert.IsTrue(pk != nil, "given player input is nil")
	//assert.IsTrue(mc.nonAuthoritative != nil, "non-authoritative data is null")
	mc.nonAuthoritative.horizontalCollision = pk.InputData.Load(packet.InputFlagHorizontalCollision)
	mc.nonAuthoritative.verticalCollision = pk.InputData.Load(packet.InputFlagVerticalCollision)

	mc.nonAuthoritative.lastPos = mc.nonAuthoritative.pos
	mc.nonAuthoritative.pos = pk.Position.Sub(playerHeightOffset)
	mc.nonAuthoritative.lastVel = mc.nonAuthoritative.vel
	mc.nonAuthoritative.vel = pk.Delta
	mc.nonAuthoritative.lastMov = mc.nonAuthoritative.mov
	mc.nonAuthoritative.mov = mc.nonAuthoritative.pos.Sub(mc.nonAuthoritative.lastPos)

	if pk.InputData.Load(packet.InputFlagStartFlying) {
		mc.nonAuthoritative.toggledFly = true
		if mc.trustFlyStatus {
			mc.flying = true
		}
	} else if pk.InputData.Load(packet.InputFlagStopFlying) {
		if mc.flying {
			mc.justDisabledFlight = true
		}
		mc.flying = false
		mc.nonAuthoritative.toggledFly = false
	}

	mc.lastRotation = mc.rotation
	mc.rotation = mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw}

	if mc.lastRotation != mc.rotation {
		delta := mc.rotation.Sub(mc.lastRotation)
		mc.mPlayer.Dbg.Notify(
			player.DebugModeRotations,
			true,
			"yawDelta=%f pitchDelta=%f headYawDelta=%f",
			delta[2], delta[0], delta[1],
		)
	}

	mc.pressingSneak = pk.InputData.Load(packet.InputFlagSneaking)
	mc.pressingSprint = pk.InputData.Load(packet.InputFlagSprintDown)

	startFlag, stopFlag := pk.InputData.Load(packet.InputFlagStartSprinting), pk.InputData.Load(packet.InputFlagStopSprinting)
	isNewVersionPlayer := mc.mPlayer.VersionInRange(player.GameVersion1_21_0, 65536)
	var needsSpeedAdjusted bool
	if startFlag && stopFlag /*&& hasForwardKeyPressed*/ {
		mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, isNewVersionPlayer, "1.21.0+ start/stop state race condition")
		needsSpeedAdjusted = isNewVersionPlayer
		/*if !mc.serverSprintApplied {
			if mc.serverSprint {
				mc.sprinting = true
				mc.airSpeed = 0.026
				mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "server sprint applied - airSpeed adjusted to 0.026")
			} else {
				mc.sprinting = false
				mc.airSpeed = 0.02
				mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "server sprint applied - airSpeed adjusted to 0.02")
			}
		}*/
		mc.sprinting = false
		mc.airSpeed = 0.02
		mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "airSpeed adjusted to 0.02")
	} else if !startFlag && !stopFlag && !mc.serverSprintApplied && mc.serverSprint != mc.sprinting {
		// TODO: Do we have to apply the speed adjustment herer?
		if mc.serverSprint {
			mc.sprinting = true
			mc.airSpeed = 0.026
			mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "server sprint applied - airSpeed adjusted to 0.026")
		} else {
			mc.sprinting = false
			mc.airSpeed = 0.02
			mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "server sprint applied - airSpeed adjusted to 0.02")
		}
	} else if startFlag /*  && !mc.sprinting && hasForwardKeyPressed*/ {
		mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, isNewVersionPlayer, "1.21.0+ starts sprint")
		mc.sprinting = true

		needsSpeedAdjusted = isNewVersionPlayer
		mc.airSpeed = 0.026
		mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "airSpeed adjusted to 0.026")
	} else if stopFlag /*&& mc.sprinting && !hasForwardKeyPressed*/ {
		mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, isNewVersionPlayer, "1.21.0+ stops sprint")
		mc.sprinting = false

		needsSpeedAdjusted = isNewVersionPlayer && !mc.serverUpdatedSpeed
		mc.airSpeed = 0.02
		mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "airSpeed adjusted to 0.02")
	}
	mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, !mc.serverSprintApplied, "server sprint not applied on current frame")
	mc.serverSprintApplied = true

	if needsSpeedAdjusted {
		mc.serverUpdatedSpeed = false
		mc.movementSpeed = mc.defaultMovementSpeed
		if mc.sprinting {
			mc.movementSpeed *= 1.3
		}
	}

	if pk.InputData.Load(packet.InputFlagStartSneaking) {
		mc.sneaking = true
	} else if pk.InputData.Load(packet.InputFlagStopSneaking) {
		mc.sneaking = false
	} else {
		mc.sneaking = pk.InputData.Load(packet.InputFlagSneakDown)
	}

	mc.mPlayer.Dbg.Notify(
		player.DebugModeMovementSim,
		true,
		"rawMoveVector=%v",
		pk.MoveVector,
	)

	maxImpulse := float32(1.0)
	/* if pk.MoveVector[0] != 0 && pk.MoveVector[1] != 0 {
		maxImpulse = game.MaxNormalizedImpulse
	} */
	if mc.mPlayer.StartUseConsumableTick != 0 {
		maxImpulse *= game.MaxConsumingImpulse
	}
	if mc.sneaking {
		maxImpulse *= game.MaxSneakImpulse
	}
	pk.MoveVector[0] = game.ClampFloat(pk.MoveVector[0], -maxImpulse, maxImpulse)
	pk.MoveVector[1] = game.ClampFloat(pk.MoveVector[1], -maxImpulse, maxImpulse)

	mc.jumping = pk.InputData.Load(packet.InputFlagStartJumping)
	mc.pressingJump = pk.InputData.Load(packet.InputFlagJumping)
	mc.jumpHeight = game.DefaultJumpHeight
	if jumpBoost, ok := mc.mPlayer.Effects().Get(packet.EffectJumpBoost); ok {
		mc.jumpHeight += float32(jumpBoost.Amplifier) * 0.1
	}

	// Jump timer resets if the jump button is not held down.
	if !mc.pressingJump {
		mc.jumpDelay = 0
	}
	mc.gravity = game.NormalGravity

	// The stop flag should be checked first, as this would indicate to us that the player is no longer gliding.
	// In the case where both flags are sent in the same tick, the gliding status will be set to false.
	if pk.InputData.Load(packet.InputFlagStopGliding) {
		mc.gliding = false
		mc.glideBoostTicks = 0
	} else if pk.InputData.Load(packet.InputFlagStartGliding) {
		mc.gliding = true
	}

	mc.impulse = pk.MoveVector.Mul(0.98)
	simulation.SimulatePlayerMovement(mc.mPlayer, mc)

	// On older versions, there seems to be a delay before the sprinting status is actually applied.
	if !isNewVersionPlayer {
		needsSpeedAdjusted = false
		if startFlag && stopFlag /*&& hasForwardKeyPressed*/ {
			mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "1.20.80- has start/stop sprint race condition")
			mc.sprinting = false
			needsSpeedAdjusted = true
		} else if startFlag /*&& !mc.sprinting && hasForwardKeyPressed*/ {
			mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "1.20.80- starts sprint")
			mc.sprinting = true
			needsSpeedAdjusted = true
		} else if stopFlag /*&& mc.sprinting && !hasForwardKeyPressed*/ {
			mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "1.20.80- stops sprint")
			mc.sprinting = false
			needsSpeedAdjusted = !mc.serverUpdatedSpeed
		}
		// Adjust the movement speed of the movement component if their sprint state changes.
		if needsSpeedAdjusted {
			mc.serverUpdatedSpeed = false
			mc.movementSpeed = mc.defaultMovementSpeed
			if mc.sprinting {
				mc.movementSpeed *= 1.3
			}
		}
	}

	// Notify any detections that need to handle knockback.
	if mc.HasKnockback() {
		for _, d := range mc.mPlayer.Detections() {
			if d, ok := d.(interface{ HandleKnockback() }); ok {
				d.HandleKnockback()
			}
		}
	}

	mc.glideBoostTicks--
	mc.ticksSinceKb++
	mc.ticksSinceTeleport++
	if mc.jumpDelay > 0 {
		mc.jumpDelay--
	}
	mc.justDisabledFlight = false
}

// ServerUpdate updates certain states of the movement component based on a packet sent by the remote server.
func (mc *AuthoritativeMovementComponent) ServerUpdate(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.MobEffect:
		if pk.EntityRuntimeID == mc.mPlayer.RuntimeId {
			mc.mPlayer.ACKs().Add(acknowledgement.NewPlayerEffectsACK(
				mc.mPlayer,
				pk.EffectType,
				pk.Amplifier,
				pk.Duration,
				pk.Operation,
			))
		}
	case *packet.MoveActorAbsolute:
		if utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
			mc.SetPendingTeleportPos(pk.Position)
			mc.AddPendingTeleport()
			mc.mPlayer.ACKs().Add(acknowledgement.NewTeleportPlayerACK(mc.mPlayer, pk.Position, utils.HasFlag(uint64(pk.Flags), packet.MoveFlagOnGround), false))
		}
	case *packet.MovePlayer:
		if pk.Mode != packet.MoveModeRotation {
			if pk.Mode == packet.MoveModeReset {
				pk.Mode = packet.MoveModeTeleport
			}

			tpPos := pk.Position.Sub(playerHeightOffset)
			mc.SetPendingTeleportPos(tpPos)
			mc.AddPendingTeleport()
			mc.mPlayer.ACKs().Add(acknowledgement.NewTeleportPlayerACK(mc.mPlayer, tpPos, pk.OnGround, pk.Mode == packet.MoveModeNormal))
		}
	case *packet.SetActorData:
		mc.mPlayer.ACKs().Add(acknowledgement.NewUpdateActorData(mc.mPlayer, pk.EntityMetadata))
	case *packet.SetActorMotion:
		networkOpts := mc.mPlayer.Opts().Network
		kbTimeout := int64(networkOpts.MaxKnockbackDelay)
		if kbTimeout < 0 {
			kbTimeout = 1_000_000_000
		}
		kbAck := acknowledgement.NewKnockbackACK(mc.mPlayer, pk.Velocity, kbTimeout)
		if cutoff := networkOpts.GlobalMovementCutoffThreshold; cutoff >= 0 && mc.mPlayer.ServerTick-mc.mPlayer.ClientTick >= int64(cutoff) {
			kbAck.Run()
		} else {
			mc.mPlayer.ACKs().Add(kbAck)
		}
	case *packet.UpdateAbilities:
		mc.mPlayer.ACKs().Add(acknowledgement.NewUpdateAbilitiesACK(mc.mPlayer, pk.AbilityData))
	case *packet.UpdateAttributes:
		mc.mPlayer.ACKs().Add(acknowledgement.NewUpdateAttributesACK(mc.mPlayer, pk.Attributes))
	default:
		mc.mPlayer.Disconnect(fmt.Sprintf(game.ErrorInternalInvalidPacketForMovementComponent, pk))
		//panic(oerror.New("movement component cannot handle %T", pk))
	}
}

// Reset is a function that resets the current movement of the movement component to the client's non-authoritative movement.
func (mc *AuthoritativeMovementComponent) Reset() {
	mc.lastPos = mc.nonAuthoritative.lastPos
	mc.pos = mc.nonAuthoritative.pos
	mc.lastVel = mc.nonAuthoritative.lastVel
	mc.vel = mc.nonAuthoritative.vel
	mc.lastMov = mc.nonAuthoritative.lastMov
	mc.mov = mc.nonAuthoritative.mov
	if mc.flying {
		mc.onGround = false
	}

	if mc.mPlayer.Opts().Movement.LimitAllVelocity {
		limitThreshold := mc.mPlayer.Opts().Movement.LimitAllVelocityThreshold
		if limitThreshold < 0 {
			limitThreshold = -limitThreshold
		}
		mc.vel[0] = game.ClampFloat(mc.vel[0], -limitThreshold, limitThreshold)
		mc.vel[1] = game.ClampFloat(mc.vel[1], -limitThreshold, limitThreshold)
		mc.vel[2] = game.ClampFloat(mc.vel[2], -limitThreshold, limitThreshold)
	}
}

// PendingCorrections returns the number of pending corrections the movement component has.
func (mc *AuthoritativeMovementComponent) PendingCorrections() int {
	return mc.pendingCorrections
}

// AddPendingCorrection increments the number of pending corrections the movement component has.
func (mc *AuthoritativeMovementComponent) AddPendingCorrection() {
	mc.pendingCorrections++
}

// RemovePendingCorrection decrements the number of pending corrections the movement component has.
func (mc *AuthoritativeMovementComponent) RemovePendingCorrection() {
	mc.pendingCorrections--
}

// InCorrectionCooldown returns true if the movement component is in a correction cooldown.
func (mc *AuthoritativeMovementComponent) InCorrectionCooldown() bool {
	return mc.inCorrectionCooldown
}

// SetCorrectionCooldown sets whether the movement component is in a correction cooldown.
func (mc *AuthoritativeMovementComponent) SetCorrectionCooldown(cooldown bool) {
	mc.inCorrectionCooldown = cooldown
}

func (mc *AuthoritativeMovementComponent) Sync() {
	if mc.mPlayer.MState.IsReplay {
		return
	}

	mc.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "correcting movement for simulation frame %d", mc.mPlayer.SimulationFrame)
	if mc.mPlayer.Dbg.Enabled(player.DebugModeMovementSim) {
		mc.mPlayer.Message("correcting movement for simulation frame %d", mc.mPlayer.SimulationFrame)
	}

	mc.AddPendingCorrection()
	mc.SetCorrectionCooldown(true)
	mc.mPlayer.ACKs().Add(acknowledgement.NewMovementCorrectionACK(mc.mPlayer))
	// Update the blocks in the world so the client can sync itself properly. We only want to update blocks that have the potential to affect the player's movement
	// (the ones they are colliding with).
	mc.mPlayer.SyncWorld()

	if !mc.mPlayer.PendingCorrectionACK {
		// Make sure all of the player's actor data is up-to-date with Oomph's prediction.
		actorData := mc.mPlayer.LastSetActorData
		actorData.Tick = mc.mPlayer.SimulationFrame
		if f, ok := actorData.EntityMetadata[entity.DataKeyFlags]; ok {
			flags := f.(int64)
			if mc.sprinting {
				flags = utils.AddFlag(flags, entity.DataFlagSprinting)
			} else {
				flags = utils.RemoveFlag(flags, entity.DataFlagSprinting)
			}
			if mc.sneaking {
				flags = utils.AddFlag(flags, entity.DataFlagSneaking)
			} else {
				flags = utils.RemoveFlag(flags, entity.DataFlagSneaking)
			}
			if mc.immobile {
				flags = utils.AddFlag(flags, entity.DataFlagImmobile)
			} else {
				flags = utils.RemoveFlag(flags, entity.DataFlagImmobile)
			}
			actorData.EntityMetadata[entity.DataKeyFlags] = flags
		}
		mc.mPlayer.SendPacketToClient(actorData)

		// Send the actual movement correction to the client.
		mc.mPlayer.SendPacketToClient(&packet.CorrectPlayerMovePrediction{
			PredictionType: packet.PredictionTypePlayer,
			Position:       mc.Pos().Add(mgl32.Vec3{0, 1.621}),
			Delta:          mc.Vel(),
			OnGround:       mc.OnGround(),
			Tick:           mc.mPlayer.SimulationFrame,
		})
		mc.mPlayer.PendingCorrectionACK = true
	}
}
