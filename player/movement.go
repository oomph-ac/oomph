package player

import (
	"fmt"
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (p *Player) updateMovementState() bool {
	var exempt bool
	if !p.ready || p.mInfo.InVoid || p.mInfo.Flying || p.gameMode > 0 || !p.inLoadedChunk {
		p.mInfo.OnGround = true
		p.mInfo.VerticallyCollided = true
		p.mInfo.ServerPredictedPosition = p.Position()
		p.mInfo.ServerMovement = p.mInfo.ClientMovement
		p.mInfo.CanExempt = true
		exempt = true
	} else {
		p.mInfo.InUnsupportedRewindScenario = false
		exempt = p.mInfo.CanExempt
		p.calculateExpectedMovement()
		p.mInfo.CanExempt = false
	}

	p.mInfo.UpdateTickStatus()
	return !exempt
}

func (p *Player) validateMovement() {
	posError, velError := p.mInfo.ServerPredictedPosition.Sub(p.Position()).LenSqr(), p.mInfo.ServerMovement.Sub(p.mInfo.ClientMovement).LenSqr()
	if posError > 0.04 || velError > 0.25 {
		p.correctMovement()
	}

	if posError <= 0.000001 {
		p.mInfo.ServerPredictedPosition = p.Position()
		p.mInfo.ServerMovement = p.mInfo.ClientMovement
	}
}

func (p *Player) correctMovement() {
	if p.mInfo.ExpectingFutureCorrection || p.mInfo.CanExempt || p.mInfo.InUnsupportedRewindScenario {
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

func (p *Player) processInput(pk *packet.PlayerAuthInput) {
	p.inputMode = pk.InputMode

	p.wMu.Lock()
	p.inLoadedChunk = world_chunkExists(p.world, p.mInfo.ServerPredictedPosition) // Not being in a loaded chunk can cause issues with movement predictions - especially when collision checks are done
	p.wMu.Unlock()

	p.Move(pk)

	p.mInfo.MoveForward = float64(pk.MoveVector.Y()) * 0.98
	p.mInfo.MoveStrafe = float64(pk.MoveVector.X()) * 0.98

	if utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting) {
		p.mInfo.Sprinting = true
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting) {
		p.mInfo.Sprinting = false
	}

	if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) {
		p.mInfo.Sneaking = true
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
		p.mInfo.Sneaking = false
	}
	p.mInfo.Jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)
	p.mInfo.SprintDown = utils.HasFlag(pk.InputData, packet.InputFlagSprintDown)
	p.mInfo.SneakDown = utils.HasFlag(pk.InputData, packet.InputFlagSneakDown) || utils.HasFlag(pk.InputData, packet.InputFlagSneakToggleDown)
	p.mInfo.JumpDown = utils.HasFlag(pk.InputData, packet.InputFlagJumpDown)
	p.mInfo.InVoid = p.Position().Y() < -35

	p.mInfo.JumpVelocity = game.DefaultJumpMotion
	p.mInfo.Speed = game.NormalMovementSpeed
	p.mInfo.Gravity = game.NormalGravity

	p.tickEffects()

	if p.mInfo.Sprinting {
		p.mInfo.Speed *= 1.3
	}

	if p.updateMovementState() {
		p.validateMovement()
	}

	p.WorldLoader().Move(p.mInfo.ServerPredictedPosition)
	r := p.conn.ChunkRadius()
	if p.inLoadedChunk {
		fmt.Println("loading!!!")
		p.WorldLoader().Load(int(math.Floor(float64(r*r) * math.Pi)))
	}

	pk.Position = game.Vec64To32(p.mInfo.ServerPredictedPosition.Add(mgl64.Vec3{0, 1.62}))
	p.mInfo.Teleporting = false
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

	if climb && (p.mInfo.HorizontallyCollided || p.mInfo.JumpDown) {
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

	p.simulateGravity()
	p.simulateHorizontalFriction(v1)
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
	moveBB = moveBB.Translate(mgl64.Vec3{0, 0, deltaZ})

	if flag && ((vel[0] != deltaX) || (vel[2] != deltaZ)) {
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
			moveBB = stepBB
			//p.SendOomphDebug(fmt.Sprint(deltaY))
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

	min, max := moveBB.Min(), moveBB.Max()
	p.mInfo.ServerPredictedPosition = mgl64.Vec3{
		(min[0] + max[0]) / 2,
		min[1] - p.mInfo.StepLenience,
		(min[2] + max[2]) / 2,
	}
	if p.mInfo.StepLenience > 1e-4 {
		p.mInfo.ServerPredictedPosition = p.Position() // TODO! __Proper__ step predictions
	}

	bb := p.AABB().Translate(p.mInfo.ServerPredictedPosition)
	blocks = utils.NearbyBBoxes(bb, p.World())
	if cube.AnyIntersections(blocks, bb) && !p.mInfo.HorizontallyCollided && !p.mInfo.VerticallyCollided {
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

	MoveForward, MoveStrafe float64
	JumpVelocity            float64
	Gravity                 float64
	Speed                   float64
	StepLenience            float64

	MotionTicks uint64
	LiquidTicks uint64

	Sneaking, SneakDown   bool
	Jumping, JumpDown     bool
	Sprinting, SprintDown bool
	Teleporting           bool
	Immobile              bool
	Flying                bool

	IsCollided, VerticallyCollided, HorizontallyCollided bool
	XCollision, ZCollision                               bool
	OnGround, LastOnGround                               bool
	InVoid                                               bool

	ClientMovement          mgl64.Vec3
	ServerSentMovement      mgl64.Vec3
	ServerMovement          mgl64.Vec3
	ServerPredictedPosition mgl64.Vec3
}

func (m *MovementInfo) UpdateServerSentVelocity(velo mgl64.Vec3) {
	m.ServerSentMovement = velo
	m.MotionTicks = 0
}

func (m *MovementInfo) UpdateTickStatus() {
	m.MotionTicks++
	m.LiquidTicks++
}
