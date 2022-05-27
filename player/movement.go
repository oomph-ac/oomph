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

	p.simulateCollisions()

	if climb && p.mInfo.HorizontallyCollided {
		p.mInfo.ServerMovement[1] = 0.2
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

	if mgl64.Abs(p.mInfo.ClientMovement[0]) < 0.0001 {
		p.mInfo.ClientMovement[0] = 0
	}
	if mgl64.Abs(p.mInfo.ClientMovement[1]) < 0.0001 {
		p.mInfo.ClientMovement[1] = 0
	}
	if mgl64.Abs(p.mInfo.ClientMovement[2]) < 0.0001 {
		p.mInfo.ClientMovement[2] = 0
	}

	p.mInfo.ServerPredictedMovement = p.mInfo.ServerMovement

	p.simulateVerticalFriction()
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
	vel := p.mInfo.ServerMovement
	deltaX, deltaY, deltaZ := vel[0], vel[1], vel[2]

	entityBBox := p.AABB().Translate(p.entity.LastPosition())
	blocks := utils.NearbyBBoxes(entityBBox.Extend(vel), p.World())

	// Check collisions on the Y axis first
	for _, blockBBox := range blocks {
		deltaY = entityBBox.YOffset(blockBBox, deltaY)
	}
	entityBBox = entityBBox.Translate(mgl64.Vec3{0, deltaY})

	// Afterward, check for collisions on the X and Z axis
	for _, blockBBox := range blocks {
		deltaX = entityBBox.XOffset(blockBBox, deltaX)
	}
	entityBBox = entityBBox.Translate(mgl64.Vec3{deltaX})
	for _, blockBBox := range blocks {
		deltaZ = entityBBox.ZOffset(blockBBox, deltaZ)
	}
	//entityBBox = entityBBox.Translate(mgl64.Vec3{0, 0, deltaZ})

	if !mgl64.FloatEqual(vel[0], deltaX) {
		p.mInfo.XCollision = true
		vel[0] = deltaX
	} else {
		p.mInfo.XCollision = false
	}

	p.mInfo.OnGround = false
	if !mgl64.FloatEqual(vel[1], deltaY) {
		p.mInfo.VerticallyCollided = true
		if vel[1] < 0 {
			p.mInfo.OnGround = true
		}
		vel[1] = deltaY
	} else {
		p.mInfo.VerticallyCollided = false
	}

	if !mgl64.FloatEqual(vel[2], deltaZ) {
		p.mInfo.ZCollision = true
		vel[2] = deltaZ
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
