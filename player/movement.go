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
	// Client returns the non-authoritative client movement sent to the server.
	Client() NonAuthoritativeMovementInfo

	// Pos returns the position of the movement component.
	Pos() mgl32.Vec3
	// LastPos returns the previous position of the movement component.
	LastPos() mgl32.Vec3
	// SetPos sets the position of the movement component.
	SetPos(pos mgl32.Vec3)

	// SlideOffset returns the slide offset of the player.
	SlideOffset() mgl32.Vec2
	// SetSlideOffset sets the slide offset of the player.
	SetSlideOffset(mgl32.Vec2)

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

	// Sprinting returns true if the movement component is sprinting.
	Sprinting() bool
	// SetSprinting sets wether or not the movement component is sprinting.
	SetSprinting(sprint bool)
	// PressingSprint returns wether or not the movement component is holding down the key bound to the sprint action.
	PressingSprint() bool

	// Jumping returns true if the movement component is expecting a jump in the current frame.
	Jumping() bool
	// PressingJump returns true if the movement component is holding down the key bound to the jump action.
	PressingJump() bool
	// JumpDelay returns the number of ticks until the movement component can make another jump.
	JumpDelay() uint64
	// SetJumpDelay sets the number of ticks until the movement component can make another jump.
	SetJumpDelay(ticks uint64)

	// Sneaking returns true if the movement component is currently sneaking.
	Sneaking() bool
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
	// TeleportSmoothed returns true if the movement component has a teleport that needs to be smoothed out.
	TeleportSmoothed() bool
	// RemainingTeleportTicks returns the amount of ticks the teleport still needs to be completed.
	RemainingTeleportTicks() int

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

	// FallDistance returns the fall distance of the movement component.
	FallDistance() float32
	// SetFallDistance sets the fall distance of the movement component.
	SetFallDistance(fallDistance float32)

	// MovementSpeed returns the movement speed of the movement component.
	MovementSpeed() float32
	// SetMovementSpeed sets the movement speed of the movement component.
	SetMovementSpeed(speed float32)
	// DefaultMovementSpeed return the movement speed the client should default to.
	DefaultMovementSpeed() float32
	// SetDefaultMovementSpeed sets the movement speed the client should default to.
	SetDefaultMovementSpeed(speed float32)

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

	// Immobile returns true if the movement component is immobile.
	Immobile() bool
	// SetImmobile sets wether or not the movement component is immobile.
	SetImmobile(immobile bool)

	// NoClip returns true if the movement component has no collisions.
	NoClip() bool
	// SetNoClip sets wether or not the movement component has no collisions.
	SetNoClip(noClip bool)

	// Gliding returns true if the movement component is gliding.
	Gliding() bool
	// SetGliding sets wether or not movement component has no collisions.
	SetGliding(gliding bool)
	// AddGlideBooster adds a glide booster to the glide booster list.
	AddGlideBooster(eid uint64, flightTicks int64)
	// GlideBoosters returns how many glide boosts should be applied to the movement component in the current simulation frame.
	GlideBoosters() (boosters int)
	// RemoveGlideBooster removes a glide booster to the glide booster list.
	RemoveGlideBooster(eid uint64)

	// CanSimulate returns true if the movement component can be simulated by the server for the current frame.
	CanSimulate() bool
	// SetCanSimulate sets wether or not the movement component can be simulated by the server for the current frame.
	SetCanSimulate(simulate bool)

	// Flying returns true if the movement component is currently flying.
	Flying() bool
	// SetFlying sets wether or not the movement component is flying.
	SetFlying(fly bool)
	// TrustFlyStatus returns wether or not the movement component can trust the fly status sent by the client.
	TrustFlyStatus() bool
	// SetTrustFlyStatus sets wether or not the movement component can trust the fly status sent by the client.
	SetTrustFlyStatus(bool)

	// Update updates the states of the movement component from the given input.
	Update(input *packet.PlayerAuthInput)
	// ServerUpdate updates certain states of the movement component based on a packet sent by the remote server.
	ServerUpdate(pk packet.Packet)

	// SetValidationThreshold sets the amount of blocks the client's position can deviate from the simulated one before a correction is required.
	SetValidationThreshold(threshold float32)
	// ValidationThreshold returnsr the amount of blocks the client's position can deviate from the simmulated one before a correction is required.
	ValidationThreshold() float32
	// Validate is a function that returns true if this movement component has a position within the given threshold of the non authoritative movement.
	Validate() bool
	// Reset is a function that resets the current movement of the movement component to the client's non-authoritative movement.
	Reset()
}

// NonAuthoritativeMovementInfo represents movement information that the player has sent to the server but has not validated/verified.
type NonAuthoritativeMovementInfo interface {
	Pos() mgl32.Vec3
	LastPos() mgl32.Vec3

	Vel() mgl32.Vec3
	LastVel() mgl32.Vec3

	Mov() mgl32.Vec3
	LastMov() mgl32.Vec3

	// ToggledFly returns wether or not the client has attempted to trigger a fly action.
	ToggledFly() bool
	// SetToggledFly sets wether or not the client has attempted to trigger a fly action.
	SetToggledFly(bool)
}

func (p *Player) SetMovement(c MovementComponent) {
	p.movement = c
}

func (p *Player) Movement() MovementComponent {
	return p.movement
}

func (p *Player) handlePlayerMovementInput(pk *packet.PlayerAuthInput) {
	p.SimulationFrame = pk.Tick
	p.ClientTick++

	p.effects.Tick()
	p.movement.Update(pk)

	// If the client's prediction of movement deviates from the server, we send a correction so that the client can re-sync.
	if !p.movement.Validate() {
		p.correctMovement()
	}

	// To prevent the server never accepting our position (PMMP), we will always set our position to the final teleport position if a teleport is in progress.
	// Otherwise, we will use the movement component's prediction.
	var finalPos mgl32.Vec3
	if p.Movement().HasTeleport() {
		finalPos = p.Movement().TeleportPos()
	} else {
		finalPos = p.Movement().Pos()
	}

	// Update the position given in this packet to what the server predicts is correct. This is used so that there isn't any weird
	// rubberbanding that is visible on the POVs of the other players if corrections are sent. This also implies the fact that
	// other players will be unable to see if another client is using a cheat to modify their movement (e.g - fly). Of course, that is
	// granted that the movement scenario is supported by Oomph.
	pk.Position = finalPos.Add(mgl32.Vec3{0, 1.621})
}

func (p *Player) correctMovement() {
	// We never want to send a correction while the player is in the middle of the teleport.
	// Any discrepencies between the client and the server will be corrected in the next frame where a
	// teleport is not occuring.

	// Update the blocks in the world so the client can sync itself properly.
	p.SyncWorld()

	p.Dbg.Notify(DebugModeMovementSim, true, "correcting movement for simulation frame %d", p.SimulationFrame)
	if p.Dbg.Enabled(DebugModeMovementSim) {
		p.Message("correcting movement for simulation frame %d", p.SimulationFrame)
	}

	p.SendPacketToClient(&packet.CorrectPlayerMovePrediction{
		PredictionType: packet.PredictionTypePlayer,
		Position:       p.movement.Pos().Add(mgl32.Vec3{0, 1.621}),
		Delta:          p.movement.Vel(),
		OnGround:       p.movement.OnGround(),
		Tick:           p.SimulationFrame,
	})
}
