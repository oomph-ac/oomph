package handler

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDMovement = "oomph:movement"

type MovementHandler struct {
	// Position and PrevPosition are the current and previous *simulated* positions of the player.
	Position, PrevPosition mgl32.Vec3
	// ClientPosition and PrevClientPosition are the current and previous *client* positions of the player.
	ClientPosition, PrevClientPosition mgl32.Vec3
	// ClientVel and PrevClientVel are the current and previous *client* velocities of the player.
	ClientVel, PrevClientVel mgl32.Vec3
	// Rotation vectors are formatted as {pitch, headYaw, yaw}
	Rotation, PrevRotation mgl32.Vec3

	// Sneaking is true if the player is sneaking.
	Sneaking bool

	// Knockback is a Vec3 of the knockback applied to the player.
	Knockback           mgl32.Vec3
	TicksSinceKnockback int

	// TeleportPos is the position the player is teleporting to.
	TeleportPos        mgl32.Vec3
	SmoothTeleport     bool
	TicksSinceTeleport int
}

func NewMovementHandler() *MovementHandler {
	return &MovementHandler{}
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

	// Update the amount of ticks since actions.
	h.TicksSinceKnockback++
	if h.TicksSinceKnockback > 0 {
		h.Knockback[0] = 0
		h.Knockback[1] = 0
		h.Knockback[2] = 0
	}

	// Update the client's own position.
	h.PrevClientPosition = h.ClientPosition
	h.ClientPosition = input.Position.Sub(mgl32.Vec3{0, 1.62})

	// Update the client's own velocity.
	h.PrevClientVel = h.ClientVel
	h.ClientVel = h.ClientPosition.Sub(h.PrevClientPosition)

	// Update the client's rotations.
	h.PrevRotation = h.Rotation
	h.Rotation = mgl32.Vec3{input.Pitch, input.HeadYaw, input.Yaw}

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
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}

		// All other modes are capable of teleporting the player.
		if pk.Mode == packet.MoveModeRotation {
			return true
		}

		// Wait for the client to acknowledge the teleport
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.teleport(pk.Position.Sub(mgl32.Vec3{0, 1.62}), pk.Mode == packet.MoveModeNormal)
		})
	case *packet.MoveActorAbsolute:

	}

	return true
}

func (MovementHandler) OnTick(p *player.Player) {}

func (h *MovementHandler) teleport(pos mgl32.Vec3, smooth bool) {
	h.TeleportPos = pos
	h.SmoothTeleport = smooth
	h.TicksSinceTeleport = -1
}

func (h *MovementHandler) knockback(kb mgl32.Vec3) {
	h.Knockback = kb
	h.TicksSinceKnockback = -1
}
