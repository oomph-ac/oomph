package player

import (
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
)

func (p *Player) updateMovementState() {
	// TODO: Movement predictions
	if !p.ready || p.mInfo.InVoid || p.mInfo.Flying || p.gameMode > 0 {
		p.mInfo.OnGround = true
		p.mInfo.VerticallyCollided = false
		p.mInfo.ServerPredictedMovement = p.mInfo.ClientMovement
		p.mInfo.ServerMovement = mgl64.Vec3{
			p.mInfo.ClientMovement.X() * 0.546,
			(p.mInfo.ClientMovement.Y() - p.mInfo.Gravity) * game.GravityMultiplier,
			p.mInfo.ClientMovement.Z() * 0.546,
		}
		p.mInfo.CanExempt = true
	} else {
		p.calculateExpectedMovement()
		p.mInfo.CanExempt = false
	}

	p.mInfo.Tick()
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
		if b, ok := p.World().Block(cube.PosFromVec3(p.Position()).Side(cube.FaceDown)).(block.Frictional); ok {
			v1 *= b.Friction()
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

	climb := utils.BlockClimbable(p.World().Block(cube.PosFromVec3(p.Entity().LastPosition())))
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

	p.simulateCollisions(0.001)

	if climb && p.mInfo.HorizontallyCollided {
		p.mInfo.ServerMovement[1] = 0.2
	}

	p.mInfo.ServerPredictedMovement = p.mInfo.ServerMovement

	p.simulateVerticalFriction()
	p.simulateHorizontalFriction()

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

func (p *Player) simulateCollisions(epsilon float64) {
	vel := p.mInfo.ServerMovement
	deltaX, deltaY, deltaZ := vel[0], vel[1], vel[2]
	movX, movY, movZ := deltaX, deltaY, deltaZ
	//fmt.Println("client wants to move", movX, movZ, "at tick", p.clientTick.Load())

	entityBBox := p.AABB().Translate(p.entity.LastPosition())
	blocks := utils.NearbyBBoxes(entityBBox.Extend(vel), p.World())

	if !mgl64.FloatEqualThreshold(deltaY, 0, epsilon) {
		// First we move the entity BBox on the Y axis.
		for _, blockBBox := range blocks {
			deltaY = entityBBox.YOffset(blockBBox, deltaY)
		}
		entityBBox = entityBBox.Translate(mgl64.Vec3{0, deltaY})
	}

	flag := p.mInfo.OnGround || (deltaY != movY && movY < 0)

	if !mgl64.FloatEqualThreshold(deltaX, 0, epsilon) {
		// Then on the X axis.
		for _, blockBBox := range blocks {
			deltaX = entityBBox.XOffset(blockBBox, deltaX)
		}
		entityBBox = entityBBox.Translate(mgl64.Vec3{deltaX})
	}

	if !mgl64.FloatEqualThreshold(deltaZ, 0, epsilon) {
		// And finally on the Z axis.
		for _, blockBBox := range blocks {
			deltaZ = entityBBox.ZOffset(blockBBox, deltaZ)
		}
	}

	if flag && (movX != deltaX || movZ != deltaZ) {
		cx, cy, cz := deltaX, deltaY, deltaZ
		deltaX, deltaY, deltaZ = movX, game.StepHeight, movZ
		entityBBox = p.AABB().Translate(p.entity.LastPosition())
		blocks := utils.NearbyBBoxes(entityBBox.Extend(mgl64.Vec3{deltaX, deltaY, deltaZ}), p.World())

		for _, blockBBox := range blocks {
			deltaY = entityBBox.YOffset(blockBBox, deltaY)
		}
		entityBBox = entityBBox.Translate(mgl64.Vec3{0, deltaY})

		if !mgl64.FloatEqualThreshold(deltaX, 0, epsilon) {
			for _, blockBBox := range blocks {
				deltaX = entityBBox.XOffset(blockBBox, deltaX)
			}
			entityBBox = entityBBox.Translate(mgl64.Vec3{deltaX})
		}

		if !mgl64.FloatEqualThreshold(deltaZ, 0, epsilon) {
			for _, blockBBox := range blocks {
				deltaZ = entityBBox.ZOffset(blockBBox, deltaZ)
			}
			entityBBox = entityBBox.Translate(mgl64.Vec3{0, 0, deltaZ})
		}

		reverseDeltaY := -deltaY
		for _, blockBBox := range blocks {
			reverseDeltaY = entityBBox.YOffset(blockBBox, reverseDeltaY)
		}
		deltaY += reverseDeltaY

		if (math.Pow(deltaX, 2) + math.Pow(deltaZ, 2)) <= (math.Pow(cx, 2) + math.Pow(cz, 2)) {
			deltaX, deltaY, deltaZ = cx, cy, cz
		} else {
			p.mInfo.StepLenience += deltaY
		}

	}

	if !mgl64.FloatEqual(vel[1], 0) {
		// The Y velocity of the player is currently not 0, meaning it is moving either up or down. We can
		// then assume the player is not currently on the ground.
		p.mInfo.OnGround = false
	}

	if !mgl64.FloatEqual(deltaX, movX) {
		p.mInfo.XCollision = true
		vel[0] = 0
	} else {
		p.mInfo.XCollision = false
	}

	if !mgl64.FloatEqual(deltaY, movY) {
		// The player either hit the ground or hit the ceiling.
		p.mInfo.VerticallyCollided = true
		if movY < 0 {
			// The player was going down, so we can assume it is now on the ground.
			p.mInfo.OnGround = true
		}
		vel[1] = 0
	} else {
		p.mInfo.VerticallyCollided = false
	}

	if !mgl64.FloatEqual(deltaZ, movZ) {
		p.mInfo.ZCollision = true
		vel[2] = 0
	} else {
		p.mInfo.ZCollision = false
	}

	p.mInfo.HorizontallyCollided = p.mInfo.XCollision || p.mInfo.ZCollision
	p.mInfo.ServerMovement = vel
}

func (p *Player) simulateVerticalFriction() {
	p.mInfo.ServerMovement[1] -= p.mInfo.Gravity
	p.mInfo.ServerMovement[1] *= 0.98
}

func (p *Player) simulateHorizontalFriction() {
	friction := 0.91
	if p.mInfo.OnGround {
		if b, ok := p.World().Block(cube.PosFromVec3(p.Position()).Side(cube.FaceDown)).(block.Frictional); ok {
			friction *= b.Friction()
		} else {
			friction *= 0.6
		}
	}
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

	MoveForward, MoveStrafe float64
	JumpVelocity            float64
	Gravity                 float64
	Speed                   float64
	StepLenience            float64

	MotionTicks int64

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
}

func (m *MovementInfo) UpdateServerSentVelocity(velo mgl64.Vec3) {
	m.ServerSentMovement = velo
	m.MotionTicks = 0
}

func (m *MovementInfo) Tick() {
	m.MotionTicks++
}
