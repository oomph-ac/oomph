package handler

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDMovement = "oomph:movement"

type MovementHandler struct {
	BoundingBox                        cube.BBox
	Position, PrevPosition             mgl32.Vec3
	ClientPosition, PrevClientPosition mgl32.Vec3
	ClientVel, PrevClientVel           mgl32.Vec3

	// Rotation vectors are formatted as {pitch, headYaw, yaw}
	Rotation, PrevRotation mgl32.Vec3

	Sneaking  bool
	Sprinting bool

	Jumping            bool
	TicksUntilNextJump int

	Knockback           mgl32.Vec3
	TicksSinceKnockback int

	TeleportPos        mgl32.Vec3
	SmoothTeleport     bool
	TicksSinceTeleport int

	// MovementSpeed is the current movement speed of the player.
	MovementSpeed float32
	// HasServerSpeed is false when the client does an action that changes their movement speed, but
	// has not recieved data from the server if their speed was actually modified. It is set to true when
	// the client acknowledges the change in speed.
	HasServerSpeed bool
	// ClientPredictsSpeed is set manually by the end-user, and is set to truew
	ClientPredictsSpeed bool
}

func NewMovementHandler() *MovementHandler {
	return &MovementHandler{
		BoundingBox: cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3),
	}
}

func (MovementHandler) ID() string {
	return HandlerIDMovement
}

func (h *MovementHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	// Update client tick and simulation frame.
	p.ClientFrame = int64(input.Tick)
	p.ClientTick++

	// Update the client's own position.
	h.PrevClientPosition = h.ClientPosition
	h.ClientPosition = input.Position.Sub(mgl32.Vec3{0, 1.62})

	// Update the client's own velocity.
	h.PrevClientVel = h.ClientVel
	h.ClientVel = h.ClientPosition.Sub(h.PrevClientPosition)

	// Update the client's rotations.
	h.PrevRotation = h.Rotation
	h.Rotation = mgl32.Vec3{input.Pitch, input.HeadYaw, input.Yaw}

	h.updateMovementStates(p, input)

	// TODO: Movement simulation.

	return true
}

func (h *MovementHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.knockback(pk.Velocity)
		})
	case *packet.MovePlayer:
		// Prevent client-side rewinding.
		pk.Tick = 0

		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}

		// All other modes should be capable of teleporting the player.
		if pk.Mode == packet.MoveModeRotation {
			return true
		}

		// Wait for the client to acknowledge the teleport
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.teleport(pk.Position.Sub(mgl32.Vec3{0, 1.62}), pk.Mode == packet.MoveModeNormal)
		})
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}

		if !utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
			return true
		}

		// Wait for the client to acknowledge the teleport
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.teleport(pk.Position, false)
		})
	}

	return true
}

func (*MovementHandler) OnTick(p *player.Player) {
}

func (*MovementHandler) Defer() {
}

// calculateClientSpeed calculates the speed of the client when it is predicting its own speed.
func (h *MovementHandler) calculateClientSpeed(p *player.Player) (speed float32) {
	speed = float32(0.1)
	if h.ClientPredictsSpeed {
		effectHandler := p.Handler(HandlerIDEffects).(*EffectsHandler)
		if spd, ok := effectHandler.Get(packet.EffectSpeed); ok {
			speed += float32(spd.Level()) * 0.02
		}
		if slw, ok := effectHandler.Get(packet.EffectSlowness); ok {
			speed -= float32(slw.Level()) * 0.015
		}
	}

	if h.Sprinting {
		speed *= 1.3
	}

	return
}

// teleport sets the teleport position of the player.
func (h *MovementHandler) teleport(pos mgl32.Vec3, smooth bool) {
	h.TeleportPos = pos
	h.SmoothTeleport = smooth
	h.TicksSinceTeleport = -1
}

// knockback sets the knockback of the player.
func (h *MovementHandler) knockback(kb mgl32.Vec3) {
	h.Knockback = kb
	h.TicksSinceKnockback = -1
}

func (h *MovementHandler) updateMovementStates(p *player.Player, pk *packet.PlayerAuthInput) {
	startFlag, stopFlag := utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting), utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting)
	needsSpeedAdjustment := false
	if startFlag && stopFlag {
		// When both the start and stop flags are found in the same tick, this usually indicates the player is horizontally collided as the client will
		// first check if the player is holding the sprint key (isn't sneaking, other conditions, etc.), and call setSprinting(true), but then see the player
		// is horizontally collided and call setSprinting(false) on the same call of onLivingUpdate()
		h.Sprinting = false
		needsSpeedAdjustment = true
	} else if startFlag {
		h.Sprinting = true
		needsSpeedAdjustment = true
	} else {
		h.Sprinting = false
		needsSpeedAdjustment = !h.HasServerSpeed
	}

	// If a speed adjustment is needed, calculate the new speed of the client.
	if needsSpeedAdjustment {
		h.MovementSpeed = h.calculateClientSpeed(p)
	}

	// Update the amount of ticks since actions.
	h.TicksSinceTeleport++
	h.TicksSinceKnockback++
	if h.TicksSinceKnockback > 0 {
		h.Knockback[0] = 0
		h.Knockback[1] = 0
		h.Knockback[2] = 0
	}
}
