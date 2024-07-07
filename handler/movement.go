package handler

import (
	"bytes"
	"fmt"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDMovement = "oomph:movement"

const (
	SimulationNormal = iota + 1
	SimulationAccountingGhostBlock
)

type MovementScenario struct {
	ID int

	Position mgl32.Vec3
	Velocity mgl32.Vec3

	Friction float32

	OffGroundTicks int64
	OnGround       bool

	CollisionX, CollisionZ bool
	VerticallyCollided     bool
	HorizontallyCollided   bool

	Climb  bool
	Jumped bool

	KnownInsideBlock bool
}

type MovementHandler struct {
	MovementScenario
	Scenarios []MovementScenario

	OutgoingCorrections   int32
	CorrectionTrustBuffer int32
	CorrectionThreshold   float32

	Width  float32
	Height float32

	PrevPosition                       mgl32.Vec3
	PrevVelocity                       mgl32.Vec3
	ClientPosition, PrevClientPosition mgl32.Vec3
	ClientVel, PrevClientVel           mgl32.Vec3
	Mov, ClientMov                     mgl32.Vec3
	PrevMov, PrevClientMov             mgl32.Vec3

	// Rotation vectors are formatted as {pitch, headYaw, yaw}
	Rotation, PrevRotation           mgl32.Vec3
	DeltaRotation, PrevDeltaRotation mgl32.Vec3

	ForwardImpulse float32
	LeftImpulse    float32

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

	MovementSpeed       float32
	AirSpeed            float32
	HasServerSpeed      bool
	ClientPredictsSpeed bool

	lastInput *packet.PlayerAuthInput

	s    Simulator
	mode player.AuthorityMode
}

func NewMovementHandler() *MovementHandler {
	return &MovementHandler{
		// TODO: Do we initally trust the fly status or not? In PocketMine at least,
		// the server doesn't seem to send back the UpdateAbillites packet.
		TrustFlyStatus:      true,
		CorrectionThreshold: 0.3,
	}
}

func DecodeMovementHandler(buf *bytes.Buffer) MovementHandler {
	h := MovementHandler{}
	h.Width = utils.LFloat32(buf.Next(4))
	h.Height = utils.LFloat32(buf.Next(4))

	// Read current scenario data
	h.Position = utils.ReadVec32(buf.Next(12))
	h.Velocity = utils.ReadVec32(buf.Next(12))
	h.OffGroundTicks = utils.LInt64(buf.Next(8))
	h.OnGround = utils.Bool(buf.Next(1))
	h.CollisionX = utils.Bool(buf.Next(1))
	h.CollisionZ = utils.Bool(buf.Next(1))
	h.VerticallyCollided = utils.Bool(buf.Next(1))
	h.HorizontallyCollided = utils.Bool(buf.Next(1))
	h.KnownInsideBlock = utils.Bool(buf.Next(1))

	// Read positions and velocities
	h.PrevPosition = utils.ReadVec32(buf.Next(12))
	h.PrevVelocity = utils.ReadVec32(buf.Next(12))
	h.ClientPosition = utils.ReadVec32(buf.Next(12))
	h.PrevClientPosition = utils.ReadVec32(buf.Next(12))
	h.ClientVel = utils.ReadVec32(buf.Next(12))
	h.PrevClientVel = utils.ReadVec32(buf.Next(12))
	h.Mov = utils.ReadVec32(buf.Next(12))
	h.ClientMov = utils.ReadVec32(buf.Next(12))
	h.PrevMov = utils.ReadVec32(buf.Next(12))
	h.PrevClientMov = utils.ReadVec32(buf.Next(12))

	// Read rotations
	h.Rotation = utils.ReadVec32(buf.Next(12))
	h.PrevRotation = utils.ReadVec32(buf.Next(12))

	// Read impulses
	h.ForwardImpulse = utils.LFloat32(buf.Next(4))
	h.LeftImpulse = utils.LFloat32(buf.Next(4))

	// Read sneaking/sprinting
	h.Sneaking = utils.Bool(buf.Next(1))
	h.SneakKeyPressed = utils.Bool(buf.Next(1))
	h.Sprinting = utils.Bool(buf.Next(1))
	h.SprintKeyPressed = utils.Bool(buf.Next(1))

	// Read gravity/step clip offset
	h.Gravity = utils.LFloat32(buf.Next(4))
	h.StepClipOffset = utils.LFloat32(buf.Next(4))

	// Read jumping states
	h.Jumping = utils.Bool(buf.Next(1))
	h.JumpKeyPressed = utils.Bool(buf.Next(1))
	h.JumpHeight = utils.LFloat32(buf.Next(4))
	h.TicksUntilNextJump = int(utils.LInt32(buf.Next(4)))

	// Read knockback
	h.Knockback = utils.ReadVec32(buf.Next(12))
	h.TicksSinceKnockback = int(utils.LInt32(buf.Next(4)))

	// Read teleport states
	h.TeleportPos = utils.ReadVec32(buf.Next(12))
	h.SmoothTeleport = utils.Bool(buf.Next(1))
	h.TeleportOnGround = utils.Bool(buf.Next(1))
	h.TicksSinceTeleport = int(utils.LInt32(buf.Next(4)))

	// Read no clip/immobile
	h.NoClip = utils.Bool(buf.Next(1))
	h.Immobile = utils.Bool(buf.Next(1))

	// Read flying states
	h.Flying = utils.Bool(buf.Next(1))
	h.ToggledFly = utils.Bool(buf.Next(1))
	h.TrustFlyStatus = utils.Bool(buf.Next(1))

	// Read speed states
	h.MovementSpeed = utils.LFloat32(buf.Next(4))
	h.AirSpeed = utils.LFloat32(buf.Next(4))
	h.HasServerSpeed = utils.Bool(buf.Next(1))
	h.ClientPredictsSpeed = utils.Bool(buf.Next(1))

	// Read authority mode
	h.mode = player.AuthorityMode(utils.LInt32(buf.Next(4)))

	if buf.Len() != 0 {
		panic(fmt.Sprintf("unexpected %d bytes left in movement handler buffer", buf.Len()))
	}
	return h
}

// Encode encodes the movement handler into a buffer. This is used for recordings/replays.
func (h *MovementHandler) Encode(buf *bytes.Buffer) {
	// Write width/height
	utils.WriteLFloat32(buf, h.Width)
	utils.WriteLFloat32(buf, h.Height)

	// Write current scenario data
	utils.WriteVec32(buf, h.Position)
	utils.WriteVec32(buf, h.Velocity)
	utils.WriteLInt64(buf, h.OffGroundTicks)
	utils.WriteBool(buf, h.OnGround)
	utils.WriteBool(buf, h.CollisionX)
	utils.WriteBool(buf, h.CollisionZ)
	utils.WriteBool(buf, h.VerticallyCollided)
	utils.WriteBool(buf, h.HorizontallyCollided)
	utils.WriteBool(buf, h.KnownInsideBlock)

	// Write position/velocity
	utils.WriteVec32(buf, h.PrevPosition)
	utils.WriteVec32(buf, h.PrevVelocity)
	utils.WriteVec32(buf, h.ClientPosition)
	utils.WriteVec32(buf, h.PrevClientPosition)
	utils.WriteVec32(buf, h.ClientVel)
	utils.WriteVec32(buf, h.PrevClientVel)
	utils.WriteVec32(buf, h.Mov)
	utils.WriteVec32(buf, h.ClientMov)
	utils.WriteVec32(buf, h.PrevMov)
	utils.WriteVec32(buf, h.PrevClientMov)

	// Write rotation
	utils.WriteVec32(buf, h.Rotation)
	utils.WriteVec32(buf, h.PrevRotation)

	// Write impulses (WASD)
	utils.WriteLFloat32(buf, h.ForwardImpulse)
	utils.WriteLFloat32(buf, h.LeftImpulse)

	// Write sneaking/sprinting
	utils.WriteBool(buf, h.Sneaking)
	utils.WriteBool(buf, h.SneakKeyPressed)
	utils.WriteBool(buf, h.Sprinting)
	utils.WriteBool(buf, h.SprintKeyPressed)

	// Write gravity/step clip offset
	utils.WriteLFloat32(buf, h.Gravity)
	utils.WriteLFloat32(buf, h.StepClipOffset)

	// Write jumping states
	utils.WriteBool(buf, h.Jumping)
	utils.WriteBool(buf, h.JumpKeyPressed)
	utils.WriteLFloat32(buf, h.JumpHeight)
	utils.WriteLInt32(buf, int32(h.TicksUntilNextJump))

	// Write knockback
	utils.WriteVec32(buf, h.Knockback)
	utils.WriteLInt32(buf, int32(h.TicksSinceKnockback))

	// Write teleport states
	utils.WriteVec32(buf, h.TeleportPos)
	utils.WriteBool(buf, h.SmoothTeleport)
	utils.WriteBool(buf, h.TeleportOnGround)
	utils.WriteLInt32(buf, int32(h.TicksSinceTeleport))

	// Write no clip/immobile
	utils.WriteBool(buf, h.NoClip)
	utils.WriteBool(buf, h.Immobile)

	// Write flying states
	utils.WriteBool(buf, h.Flying)
	utils.WriteBool(buf, h.ToggledFly)
	utils.WriteBool(buf, h.TrustFlyStatus)

	// Write speed states
	utils.WriteLFloat32(buf, h.MovementSpeed)
	utils.WriteLFloat32(buf, h.AirSpeed)
	utils.WriteBool(buf, h.HasServerSpeed)
	utils.WriteBool(buf, h.ClientPredictsSpeed)

	// Write authority mode
	utils.WriteLInt32(buf, int32(h.mode))
}

func (MovementHandler) ID() string {
	return HandlerIDMovement
}

// CorrectMovement sends a movement correction to the client.
func (h *MovementHandler) CorrectMovement(p *player.Player) {
	if h.StepClipOffset > 0 {
		return
	}

	h.OutgoingCorrections++
	p.SendPacketToClient(&packet.CorrectPlayerMovePrediction{
		PredictionType: packet.PredictionTypePlayer,
		Position:       h.Position.Add(mgl32.Vec3{0, 1.6201}),
		Delta:          h.Velocity,
		OnGround:       h.OnGround,
		Tick:           uint64(p.ClientFrame),
	})

	p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
		ack.AckPlayerRecieveCorrection,
	))
}

// RevertMovement sends a teleport packet to the client to revert the player's position.
func (h *MovementHandler) RevertMovement(p *player.Player, pos mgl32.Vec3) {
	p.SendPacketToClient(&packet.MovePlayer{
		EntityRuntimeID: p.RuntimeId,
		Mode:            packet.MoveModeTeleport,
		Position:        pos.Add(mgl32.Vec3{0, 1.62}),
		OnGround:        h.OnGround,

		Pitch:   h.Rotation.X(),
		HeadYaw: h.Rotation.Y(),
		Yaw:     h.Rotation.Z(),
	})

	p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
		ack.AckPlayerTeleport,
		pos,
		h.OnGround,
		false,
	))

	p.SendPacketToClient(&packet.SetActorMotion{
		EntityRuntimeID: p.RuntimeId,
		Velocity:        utils.EmptyVec32,
		Tick:            0,
	})
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

	h.PrevClientMov = h.ClientMov
	h.ClientMov = h.ClientPosition.Sub(h.PrevClientPosition)

	// Update the client's own velocity.
	h.PrevClientVel = h.ClientVel
	h.ClientVel = input.Delta

	// Update the client's rotations.
	h.PrevRotation = h.Rotation
	h.Rotation = mgl32.Vec3{input.Pitch, input.HeadYaw, input.Yaw}

	h.PrevDeltaRotation = h.DeltaRotation
	h.DeltaRotation = game.AbsVec32(h.Rotation.Sub(h.PrevRotation))
	p.Dbg.Notify(
		player.DebugModeRotations,
		h.DeltaRotation.LenSqr() > 1e-10,
		"dP=%f dHY=%f dY=%f",
		game.Round32(h.DeltaRotation[0], 4),
		game.Round32(h.DeltaRotation[1], 4),
		game.Round32(h.DeltaRotation[2], 4),
	)

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
	if p.MovementMode != player.AuthorityModeNone {
		h.s.Simulate(p)
	}
	h.TicksUntilNextJump--

	// If we are using full authority, we never send the client's position (unless the movement scenario is unsupported).
	if p.MovementMode == player.AuthorityModeComplete {
		input.Position = h.Position.Add(mgl32.Vec3{0, 1.62})
	}

	h.lastInput = input
	return true
}

func (h *MovementHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.SetActorData:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}
		pk.Tick = 0 // prevent rewind

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckPlayerUpdateActorData,
			pk.EntityMetadata,
		))
	case *packet.UpdateAbilities:
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckPlayerUpdateAbilities,
			pk.AbilityData.Layers,
		))
	case *packet.UpdateAttributes:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}
		pk.Tick = 0 // prevent rewind

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckPlayerUpdateAttributes,
			pk.Attributes,
		))
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.RuntimeId {
			return false
		}

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckPlayerUpdateKnockback,
			pk.Velocity,
		))
	case *packet.MovePlayer:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}
		pk.Tick = 0 // prevent rewind

		// Wait for the client to acknowledge the teleport.
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckPlayerTeleport,
			pk.Position.Sub(mgl32.Vec3{0, 1.62}),
			pk.OnGround,
			pk.Mode == packet.MoveModeNormal,
		))
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId {
			return true
		}

		if !utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
			return true
		}

		// Wait for the client to acknowledge the teleport.
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).Add(ack.New(
			ack.AckPlayerTeleport,
			pk.Position,
			utils.HasFlag(uint64(pk.Flags), packet.MoveFlagOnGround),
			false,
		))
	}

	return true
}

func (*MovementHandler) OnTick(p *player.Player) {
}

func (h *MovementHandler) Defer() {
	if h.mode != player.AuthorityModeSemi {
		return
	}

	// Detections may want to correct the player's movement on semi-authoritative mode, and we
	// have to set the input position to the correct position.
	score := h.CorrectionTrustBuffer
	h.CorrectionTrustBuffer--
	if h.OutgoingCorrections == 0 && score <= 0 {
		h.Reset()
	} else {
		h.lastInput.Position = h.Position.Add(mgl32.Vec3{0, 1.62})
	}
}

func (h *MovementHandler) Reset() {
	//dev := h.Position.Sub(h.ClientPosition)
	if h.TicksSinceTeleport < h.TeleportTicks() || h.StepClipOffset > 0 {
		return
	}

	h.Velocity = h.ClientVel
	h.PrevVelocity = h.PrevClientVel
	h.Position = h.ClientPosition
	h.PrevPosition = h.PrevClientPosition
}

func (h *MovementHandler) Simulate(s Simulator) {
	h.s = s
}

func (h *MovementHandler) Simulator() Simulator {
	return h.s
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
	).GrowVec3(mgl32.Vec3{-0.001, 0, -0.001})
}

func (h *MovementHandler) HandleAttribute(n string, list []protocol.Attribute, f func(protocol.Attribute)) {
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

// Teleport sets the Teleport position of the player.
func (h *MovementHandler) Teleport(pos mgl32.Vec3, ground, smooth bool) {
	h.TeleportPos = pos
	h.TeleportOnGround = ground
	h.SmoothTeleport = smooth
	h.TicksSinceTeleport = -1
}

// TeleportTicks returns the ticks until a teleport is complete.
func (h *MovementHandler) TeleportTicks() int {
	if h.SmoothTeleport {
		return 4
	}
	return 1
}

// SetKnockback sets the knockback of the player.
func (h *MovementHandler) SetKnockback(kb mgl32.Vec3) {
	h.Knockback = kb
	h.TicksSinceKnockback = -1
}

func (h *MovementHandler) updateMovementStates(p *player.Player, pk *packet.PlayerAuthInput) {
	h.ForwardImpulse = pk.MoveVector.Y() * 0.98
	h.LeftImpulse = pk.MoveVector.X() * 0.98

	if utils.HasFlag(pk.InputData, packet.InputFlagStartFlying) {
		h.ToggledFly = true
		if h.TrustFlyStatus {
			h.Flying = true
		}
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopFlying) {
		h.Flying = false
		h.ToggledFly = false
	}

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

	if je, ok := p.Handler(HandlerIDEffects).(*EffectsHandler).Get(packet.EffectJumpBoost); ok {
		h.JumpHeight += float32(je.Level()) * 0.1
	}

	// Jump timer resets if the jump button is not held down.
	if !h.JumpKeyPressed {
		h.TicksUntilNextJump = 0
	}

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
