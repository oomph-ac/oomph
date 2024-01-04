package player

import (
	"github.com/go-gl/mathgl/mgl32"
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
}

func (MovementHandler) ID() string {
	return HandlerIDMovement
}

func (h *MovementHandler) HandleClientPacket(pk packet.Packet, p *Player) bool {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	// Update client tick and simulation frame.
	p.clientFrame = int64(input.Tick)
	p.clientTick++

	// Update the amount of ticks since actions.
	h.TicksSinceKnockback++
	if h.TicksSinceKnockback > 0 {
		h.Knockback = mgl32.Vec3{0, 0, 0}
	}

	// Update the client's own position.
	h.PrevClientPosition = h.ClientPosition
	h.ClientPosition = input.Position.Sub(mgl32.Vec3{0, 1.62})

	// Update the client's own velocity.
	h.PrevClientVel = h.ClientVel
	h.ClientVel = h.ClientPosition.Sub(h.PrevClientPosition)
	//h.ClientVel = input.Delta

	// Update the client's rotations.
	h.PrevRotation = h.Rotation
	h.Rotation = mgl32.Vec3{input.Pitch, input.HeadYaw, input.Yaw}

	// TODO: Movement simulation.

	return true
}

func (h *MovementHandler) HandleServerPacket(pk packet.Packet, p *Player) bool {
	switch pk := pk.(type) {
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.runtimeId {
			return true
		}

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.Knockback = pk.Velocity
			h.TicksSinceKnockback = -1
		})
	}

	return true
}

func (MovementHandler) OnTick(p *Player) {}
