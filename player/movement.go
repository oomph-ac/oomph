package player

import (
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (p *Player) updateMovementState() {
	if !p.ready || p.mInfo.InVoid || p.mInfo.Flying || p.gameMode > 0 || p.mInfo.Teleporting || !p.inLoadedChunk {
		p.mInfo.OnGround = true
		p.mInfo.VerticallyCollided = true
		p.mInfo.ServerPredictedMovement = p.mInfo.ClientMovement
		p.mInfo.ServerPredictedPosition = p.Position()
		p.mInfo.ServerMovement = mgl64.Vec3{
			p.mInfo.ClientMovement.X() * 0.546,
			(p.mInfo.ClientMovement.Y() - p.mInfo.Gravity) * game.GravityMultiplier,
			p.mInfo.ClientMovement.Z() * 0.546,
		}
		p.mInfo.CanExempt = true
	} else {
		p.mInfo.InUnsupportedRewindScenario = false
		p.calculateExpectedMovement()
		p.mInfo.CanExempt = p.mInfo.StepLenience > 1e-4
	}

	p.mInfo.UpdateTickStatus()
}

func (p *Player) validateMovement() {
	mError := p.mInfo.ClientMovement.Sub(p.mInfo.ServerPredictedMovement)
	posError := p.Position().Sub(p.mInfo.ServerPredictedPosition)
	if mgl64.Abs(mError[0]) > 1e-3 || mgl64.Abs(mError[1]) > 1e-4 || mgl64.Abs(mError[2]) > 1e-3 {
		p.mInfo.TrustedPositionCredits *= 0.85 // Reduce the amount of trusted position credits - the client has shown it cannot be trusted as of now
		if mError.LenSqr() > 0.04 {
			p.correctMovement()
		}
	} else if !p.mInfo.ExpectingFutureCorrection {
		p.mInfo.TrustedPositionCredits++
		if p.mInfo.TrustedPositionCredits >= 40 {
			// Since the client's movement is around the same as the server's (and they have built up to a credible score), we can assume their movement is legitimate
			// and set the servers predicted position to the client's.
			p.mInfo.ServerPredictedPosition = p.Position()
		}
	}

	// This scenario would happen when the movement is within a threshold to not be corrected, but is high enough
	// the threshold to make small positional differences every tick, leading up to a big difference in the end. Although this
	// could be fixed by correcting player movement when a much lower threshold is met, but in the off chance that the prediction is slightly
	// off, this would make the player's movement flucuate and feel "laggy".
	if posError.Len() > 3 {
		p.correctMovement()
	}
}

func (p *Player) correctMovement() {
	if p.mInfo.ExpectingFutureCorrection || p.mInfo.CanExempt {
		return
	}
	if p.mInfo.InUnsupportedRewindScenario {
		p.SendOomphDebug("unable to rewind due to unsupported rewind scenario")
		return
	}
	p.mInfo.ExpectingFutureCorrection = true
	pos, delta := p.mInfo.ServerPredictedPosition, p.mInfo.ServerMovement
	pk := &packet.CorrectPlayerMovePrediction{
		Position: game.Vec64To32(pos.Add(mgl64.Vec3{0, 1.62})),
		Delta:    game.Vec64To32(delta),
		OnGround: p.mInfo.OnGround,
		Tick:     p.ClientFrame(),
	}
	p.conn.WritePacket(pk)
	p.Acknowledgement(func() {
		p.mInfo.ExpectingFutureCorrection = false
	}, false)
}

func (p *Player) calculateExpectedMovement() {
	if p.mInfo.MotionTicks == 0 {
		p.mInfo.ServerMovement = p.mInfo.ServerSentMovement
		if p.mInfo.Jumping {
			p.mInfo.ServerMovement[1] = p.mInfo.JumpVelocity
		}
	}

	if p.mInfo.Jumping && p.mInfo.OnGround {
		p.simulateJump()
	}

	v1 := 0.91
	if p.mInfo.OnGround {
		if b, ok := p.World().Block(cube.PosFromVec3(p.mInfo.ServerPredictedPosition).Side(cube.FaceDown)).(block.Frictional); ok {
			v1 *= b.Friction()
			p.mInfo.InUnsupportedRewindScenario = true
		} else {
			v1 *= 0.6
		}
	}

	var v3 float64
	if p.mInfo.OnGround {
		v3 = p.mInfo.Speed * math.Pow((0.91*0.6)/v1, 3)
	} else if p.mInfo.Sprinting {
		v3 = 0.026
	} else {
		v3 = 0.02
	}

	p.simulateAddedMovementForce(v3)

	climb := utils.BlockClimbable(p.World().Block(cube.PosFromVec3(p.mInfo.ServerPredictedPosition)))
	if climb {
		p.mInfo.ServerMovement[0] = game.ClampFloat(p.mInfo.ServerMovement.X(), -0.2, 0.2)
		p.mInfo.ServerMovement[2] = game.ClampFloat(p.mInfo.ServerMovement.Z(), -0.2, 0.2)
		if p.mInfo.ServerMovement[1] < -0.2 {
			p.mInfo.ServerMovement[1] = -0.2
		}
		if p.mInfo.Sneaking && p.mInfo.ServerMovement.Y() < 0 {
			p.mInfo.ServerMovement[1] = 0
		}
	}

	p.simulateCollisions()

	if climb && p.mInfo.HorizontallyCollided {
		p.mInfo.ServerMovement[1] = 0.3
	}

	if mgl64.Abs(p.mInfo.ServerMovement[0]) < 0.0001 {
		p.mInfo.ServerMovement[0] = 0
	}
	if mgl64.Abs(p.mInfo.ServerMovement[1]) < 0.0001 {
		p.mInfo.ServerMovement[1] = 0
	}
	if mgl64.Abs(p.mInfo.ServerMovement[2]) < 0.0001 {
		p.mInfo.ServerMovement[2] = 0
	}

	p.mInfo.ServerPredictedMovement = p.mInfo.ServerMovement
	p.simulateGravity()
	p.simulateHorizontalFriction(v1)
	if p.mInfo.StepLenience > 1e-4 {
		p.mInfo.ServerPredictedMovement[1] += p.mInfo.StepLenience
	}
}

func (p *Player) simulateAddedMovementForce(f float64) {
	v := math.Pow(p.mInfo.MoveForward, 2) + math.Pow(p.mInfo.MoveStrafe, 2)
	if v >= 1e-4 {
		v = math.Sqrt(v)
		if v < 1 {
			v = 1
		}
		v = f / v
		mf, ms := p.mInfo.MoveForward*v, p.mInfo.MoveStrafe*v
		v2, v3 := game.MCSin(p.entity.Rotation().Z()*math.Pi/180), game.MCCos(p.entity.Rotation().Z()*math.Pi/180)
		p.mInfo.ServerMovement[0] += ms*v3 - mf*v2
		p.mInfo.ServerMovement[2] += ms*v2 + mf*v3
	}
}

func (p *Player) simulateCollisions() {
	p.mInfo.StepLenience *= 0.4
	if p.mInfo.StepLenience <= 1e-4 {
		p.mInfo.StepLenience = 0
	}

	vel := p.mInfo.ServerMovement
	deltaX, deltaY, deltaZ := vel[0], vel[1], vel[2]

	moveBB := p.AABB().Translate(p.mInfo.ServerPredictedPosition)
	cloneBB := moveBB
	blocks := utils.NearbyBBoxes(cloneBB.Extend(vel), p.World())

	if p.mInfo.OnGround && p.mInfo.Sneaking {
		mov := 0.05
		for ; deltaX != 0.0 && len(utils.NearbyBBoxes(moveBB.Translate(mgl64.Vec3{deltaX, -1, 0}), p.World())) == 0; vel[0] = deltaX {
			if deltaX < mov && deltaX >= -mov {
				deltaX = 0
			} else if deltaX > 0 {
				deltaX -= mov
			} else {
				deltaX += mov
			}
		}
		for ; deltaZ != 0.0 && len(utils.NearbyBBoxes(moveBB.Translate(mgl64.Vec3{0, -1, deltaZ}), p.World())) == 0; vel[2] = deltaZ {
			if deltaZ < mov && deltaZ >= -mov {
				deltaZ = 0
			} else if deltaZ > 0 {
				deltaZ -= mov
			} else {
				deltaZ += mov
			}
		}
	}

	// Check collisions on the Y axis first
	for _, blockBBox := range blocks {
		deltaY = moveBB.YOffset(blockBBox, deltaY)
	}
	moveBB = moveBB.Translate(mgl64.Vec3{0, deltaY})

	flag := p.mInfo.OnGround || (vel[1] != deltaY && vel[1] < 0)

	// Afterward, check for collisions on the X and Z axis
	for _, blockBBox := range blocks {
		deltaX = moveBB.XOffset(blockBBox, deltaX)
	}
	moveBB = moveBB.Translate(mgl64.Vec3{deltaX})
	for _, blockBBox := range blocks {
		deltaZ = moveBB.ZOffset(blockBBox, deltaZ)
	}

	if flag && (vel[0] != deltaX || vel[2] != deltaZ) {
		cx, cy, cz := deltaX, deltaY, deltaZ
		deltaX, deltaY, deltaZ = vel[0], game.StepHeight, vel[2]

		stepBB := p.AABB().Translate(p.mInfo.ServerPredictedPosition)
		cloneBB = stepBB
		blocks = utils.NearbyBBoxes(cloneBB.Extend(mgl64.Vec3{deltaX, deltaY, deltaZ}), p.World())

		for _, blockBBox := range blocks {
			deltaY = stepBB.YOffset(blockBBox, deltaY)
		}
		stepBB = stepBB.Translate(mgl64.Vec3{0, deltaY})

		for _, blockBBox := range blocks {
			deltaX = stepBB.XOffset(blockBBox, deltaX)
		}
		stepBB = stepBB.Translate(mgl64.Vec3{deltaX})
		for _, blockBBox := range blocks {
			deltaZ = stepBB.ZOffset(blockBBox, deltaZ)
		}
		stepBB = stepBB.Translate(mgl64.Vec3{0, 0, deltaZ})

		reverseDeltaY := -deltaY
		for _, blockBBox := range blocks {
			reverseDeltaY = stepBB.YOffset(blockBBox, reverseDeltaY)
		}
		deltaY += reverseDeltaY

		if (math.Pow(cx, 2) + math.Pow(cz, 2)) >= (math.Pow(deltaX, 2) + math.Pow(deltaZ, 2)) {
			deltaX, deltaY, deltaZ = cx, cy, cz
		} else {
			p.mInfo.StepLenience += deltaY
			deltaY = cy
		}
	}

	if !mgl64.FloatEqual(vel[1], deltaY) {
		p.mInfo.VerticallyCollided = true
		if vel[1] < 0 {
			p.mInfo.OnGround = true
		} else {
			p.mInfo.OnGround = false
		}
		vel[1] = deltaY
	} else {
		p.mInfo.OnGround = false
		p.mInfo.VerticallyCollided = false
	}

	if !mgl64.FloatEqual(vel[0], deltaX) {
		p.mInfo.XCollision = true
		vel[0] = deltaX
	} else {
		p.mInfo.XCollision = false
	}

	if !mgl64.FloatEqual(vel[2], deltaZ) {
		p.mInfo.ZCollision = true
		vel[2] = deltaZ
	} else {
		p.mInfo.ZCollision = false
	}

	p.mInfo.HorizontallyCollided = p.mInfo.XCollision || p.mInfo.ZCollision
	p.mInfo.ServerMovement = vel

	p.mInfo.ServerPredictedPosition = p.mInfo.ServerPredictedPosition.Add(vel)

	bb := p.AABB().Translate(p.mInfo.ServerPredictedPosition)
	blocks = utils.NearbyBBoxes(bb, p.World())
	if cube.AnyIntersections(blocks, bb) {
		p.mInfo.InUnsupportedRewindScenario = true
	}
}

func (p *Player) simulateGravity() {
	p.mInfo.ServerMovement[1] -= p.mInfo.Gravity
	p.mInfo.ServerMovement[1] *= 0.98
}

func (p *Player) simulateHorizontalFriction(friction float64) {
	p.mInfo.ServerMovement[0] *= friction
	p.mInfo.ServerMovement[2] *= friction
}

func (p *Player) simulateJump() {
	p.mInfo.ServerMovement[1] = p.mInfo.JumpVelocity
	if p.mInfo.Sprinting {
		force := p.entity.Rotation().Z() * 0.017453292
		p.mInfo.ServerMovement[0] -= game.MCSin(force) * 0.2
		p.mInfo.ServerMovement[2] += game.MCCos(force) * 0.2
	}
}

type MovementInfo struct {
	CanExempt bool

	InUnsupportedRewindScenario bool
	ExpectingFutureCorrection   bool
	TrustedPositionCredits      float64

	MoveForward, MoveStrafe float64
	JumpVelocity            float64
	Gravity                 float64
	Speed                   float64
	StepLenience            float64

	MotionTicks uint64
	LiquidTicks uint64

	Sneaking, Sprinting bool
	Jumping             bool
	Teleporting         bool
	Immobile            bool
	Flying              bool

	IsCollided, VerticallyCollided, HorizontallyCollided bool
	XCollision, ZCollision                               bool
	OnGround, LastOnGround                               bool
	InVoid                                               bool

	ClientMovement          mgl64.Vec3
	ServerSentMovement      mgl64.Vec3
	ServerMovement          mgl64.Vec3
	ServerPredictedMovement mgl64.Vec3
	ServerPredictedPosition mgl64.Vec3

	InputQueue            []*packet.PlayerAuthInput
	LastRecievedInput     *packet.PlayerAuthInput
	FixedInputSize        int
	ProcessedInputsOnTick int
	HasMissingInput       bool
	FilledQueueTicks      int
}

func (m *MovementInfo) AddQueuedInput(input *packet.PlayerAuthInput) {
	m.InputQueue = append(m.InputQueue, input)
	if len(m.InputQueue) > m.FixedInputSize {
		m.InputQueue = m.InputQueue[1:]
	}
}

func (m *MovementInfo) UpdateInputStatus() {
	m.HasMissingInput = len(m.InputQueue) == 0
	if m.HasMissingInput {
		m.ProcessedInputsOnTick++
	}
}

func (m *MovementInfo) GetQueuedInputs() (inputs []*packet.PlayerAuthInput) {
	processed := m.ProcessedInputsOnTick
	for processed > 0 {
		if len(m.InputQueue) > 0 {
			m.LastRecievedInput = m.InputQueue[0]
			inputs = append(inputs, m.LastRecievedInput)
			m.InputQueue = m.InputQueue[1:]
			m.ProcessedInputsOnTick--
		} else {
			inputs = append(inputs, m.LastRecievedInput)
			break
		}
		processed--
	}
	if m.ProcessedInputsOnTick < 1 {
		m.ProcessedInputsOnTick = 1
	}
	remaining := len(m.InputQueue)
	if remaining != 0 {
		m.FilledQueueTicks++
		if m.FilledQueueTicks == 20 {
			m.ProcessedInputsOnTick++
			m.FilledQueueTicks = 0
		}
	} else {
		m.FilledQueueTicks = 0
	}
	return inputs
}

func (m *MovementInfo) UpdateServerSentVelocity(velo mgl64.Vec3) {
	m.ServerSentMovement = velo
	m.MotionTicks = 0
}

func (m *MovementInfo) UpdateTickStatus() {
	m.MotionTicks++
	m.LiquidTicks++
}
