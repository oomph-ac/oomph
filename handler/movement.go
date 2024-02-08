package handler

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDMovement = "oomph:movement"

type MovementHandler struct {
	Width  float32
	Height float32

	Position, PrevPosition             mgl32.Vec3
	Velocity, PrevVelocity             mgl32.Vec3
	ClientPosition, PrevClientPosition mgl32.Vec3
	ClientVel, PrevClientVel           mgl32.Vec3
	Mov, ClientMov                     mgl32.Vec3

	// Rotation vectors are formatted as {pitch, headYaw, yaw}
	Rotation, PrevRotation mgl32.Vec3

	ForwardImpulse float32
	LeftImpulse    float32

	CollisionX, CollisionZ bool
	VerticallyCollided     bool
	HorizontallyCollided   bool

	OnGround       bool
	OffGroundTicks int64

	KnownInsideBlock bool

	Sneaking        bool
	SneakKeyPressed bool

	Sprinting        bool
	SprintKeyPressed bool

	Gravity        float32
	StepClipOffset float32

	Jumping            bool
	JumpKeyPressed     bool
	JumpHeight         float32
	TicksUntilNextJump int

	Knockback           mgl32.Vec3
	TicksSinceKnockback int

	TeleportPos        mgl32.Vec3
	SmoothTeleport     bool
	TeleportOnGround   bool
	TicksSinceTeleport int

	NoClip   bool
	Immobile bool

	Flying         bool
	ToggledFly     bool
	TrustFlyStatus bool

	// MovementSpeed is the current movement speed of the player.
	MovementSpeed float32
	AirSpeed      float32
	// HasServerSpeed is false when the client does an action that changes their movement speed, but
	// has not recieved data from the server if their speed was actually modified. It is set to true when
	// the client acknowledges the change in speed.
	HasServerSpeed bool
	// ClientPredictsSpeed is set manually by the end-user, and is set to truew
	ClientPredictsSpeed bool

	// s is the simulator that will be used for movement simulations. It can be set via. UseSimulator()
	s    Simulator
	mode player.AuthorityMode
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
	h.mode = p.MovementMode

	// Update client tick and simulation frame.
	p.ClientFrame = int64(input.Tick)
	p.ClientTick++

	// Update the client's own position.
	h.PrevClientPosition = h.ClientPosition
	h.ClientPosition = input.Position.Sub(mgl32.Vec3{0, 1.62})
	h.ClientMov = h.ClientPosition.Sub(h.PrevClientPosition)

	// Update the client's own velocity.
	h.PrevClientVel = h.ClientVel
	h.ClientVel = input.Delta

	// Update the client's rotations.
	h.PrevRotation = h.Rotation
	h.Rotation = mgl32.Vec3{input.Pitch, input.HeadYaw, input.Yaw}

	defer func() {
		if h.OnGround {
			h.OffGroundTicks = 0
			return
		}

		h.OffGroundTicks++
	}()

	h.updateMovementStates(p, input)
	if h.s == nil {
		panic(oerror.New("simulator not set in movement handler"))
	}

	// Run the movement simulation.
	h.s.Simulate(p)
	h.TicksUntilNextJump--
	return true
}

func (h *MovementHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.SetActorData:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}
		pk.Tick = 0 // prevent rewind

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			width, widthExists := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]
			height, heightExists := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]
			if !widthExists {
				width = h.Width
			}
			if !heightExists {
				height = h.Height
			}

			h.Width = width.(float32)
			h.Height = height.(float32)

			f, ok := pk.EntityMetadata[entity.DataKeyFlags]
			if !ok {
				return
			}

			flags := f.(int64)
			h.Sprinting = utils.HasDataFlag(entity.DataFlagSprinting, flags)
			h.Sneaking = utils.HasDataFlag(entity.DataFlagSneaking, flags)
			h.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, flags)
		})
	case *packet.UpdateAbilities:
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			for _, l := range pk.AbilityData.Layers {
				h.NoClip = utils.HasFlag(uint64(l.Values), protocol.AbilityNoClip)
				h.Flying = utils.HasFlag(uint64(l.Values), protocol.AbilityFlying) || h.NoClip
				mayFly := utils.HasFlag(uint64(l.Values), protocol.AbilityMayFly)

				if h.ToggledFly {
					// If the player toggled flight, but the server did not allow it, we longer trust
					// their flight status. This is done to ensure players that have permission to fly
					// are able to do so w/o any movement corrections, but players that do not have permission
					// to do so aren't able to bypass movement predictions with it.
					h.TrustFlyStatus = h.Flying || mayFly
				}
				h.ToggledFly = false
			}
		})
	case *packet.UpdateAttributes:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}
		pk.Tick = 0 // prevent rewind

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.handleAttribute("minecraft:movement", pk.Attributes, func(attr protocol.Attribute) {
				h.HasServerSpeed = true
				h.MovementSpeed = float32(attr.Value)
			})
			h.handleAttribute("minecraft:health", pk.Attributes, func(attr protocol.Attribute) {
				p.Alive = attr.Value > 0
			})
		})
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.RuntimeId {
			return false
		}

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.knockback(pk.Velocity)
		})
	case *packet.MovePlayer:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}
		pk.Tick = 0 // prevent rewind

		// Wait for the client to acknowledge the teleport.
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.teleport(pk.Position.Sub(mgl32.Vec3{0, 1.62}), pk.OnGround, pk.Mode == packet.MoveModeNormal)
		})
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}

		if !utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
			return true
		}

		// Wait for the client to acknowledge the teleport.
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			h.teleport(pk.Position, utils.HasFlag(uint64(pk.Flags), packet.MoveFlagOnGround), false)
		})
	}

	return true
}

func (*MovementHandler) OnTick(p *player.Player) {
}

func (h *MovementHandler) Defer() {
	if h.mode == player.AuthorityModeSemi {
		h.Velocity = h.ClientVel
		h.Position = h.ClientPosition
	}
}

func (h *MovementHandler) Simulate(s Simulator) {
	h.s = s
}

func (h *MovementHandler) BoundingBox() cube.BBox {
	pos := h.Position
	//pos[1] += h.StepClipOffset

	return cube.Box(
		pos.X()-(h.Width/2),
		pos.Y(),
		pos.Z()-(h.Width/2),
		pos.X()+(h.Width/2),
		pos.Y()+h.Height,
		pos.Z()+(h.Width/2),
	)
}

func (h *MovementHandler) handleAttribute(n string, list []protocol.Attribute, f func(protocol.Attribute)) {
	for _, attr := range list {
		if attr.Name == n {
			f(attr)
			return
		}
	}
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
func (h *MovementHandler) teleport(pos mgl32.Vec3, ground, smooth bool) {
	h.TeleportPos = pos
	h.TeleportOnGround = ground
	h.SmoothTeleport = smooth
	h.TicksSinceTeleport = -1
}

// knockback sets the knockback of the player.
func (h *MovementHandler) knockback(kb mgl32.Vec3) {
	h.Knockback = kb
	h.TicksSinceKnockback = -1
}

func (h *MovementHandler) updateMovementStates(p *player.Player, pk *packet.PlayerAuthInput) {
	h.ForwardImpulse = pk.MoveVector.Y() * 0.98
	h.LeftImpulse = pk.MoveVector.X() * 0.98

	startFlag, stopFlag := utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting), utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting)
	needsSpeedAdjustment := false
	if startFlag && stopFlag {
		// When both the start and stop flags are found in the same tick, this usually indicates the player is horizontally collided as the client will
		// first check if the player is holding the sprint key (isn't sneaking, other conditions, etc.), and call setSprinting(true), but then see the player
		// is horizontally collided and call setSprinting(false) on the same call of onLivingUpdate()
		h.Sprinting = false
		h.HasServerSpeed = false
		needsSpeedAdjustment = true
	} else if startFlag {
		h.Sprinting = true
		h.HasServerSpeed = false
		needsSpeedAdjustment = true
	} else if stopFlag {
		h.Sprinting = false
		needsSpeedAdjustment = !h.HasServerSpeed
	}
	h.SprintKeyPressed = utils.HasFlag(pk.InputData, packet.InputFlagSprinting)

	if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) {
		h.Sneaking = true
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
		h.Sneaking = false
	}
	h.SneakKeyPressed = utils.HasFlag(pk.InputData, packet.InputFlagSneaking)

	h.Jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)
	h.JumpKeyPressed = utils.HasFlag(pk.InputData, packet.InputFlagJumping)
	h.JumpHeight = game.DefaultJumpHeight

	// If a speed adjustment is needed, calculate the new speed of the client.
	if needsSpeedAdjustment {
		h.MovementSpeed = h.calculateClientSpeed(p)
	}

	h.Gravity = game.NormalGravity
	h.AirSpeed = 0.02
	if h.Sprinting {
		h.AirSpeed += 0.006
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
