package session

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/utils"
	"math"
)

type Movement struct {
	Session                       *Session
	Motion                        mgl64.Vec3
	ServerSentMotion              mgl32.Vec3
	ServerPredictedMotion         mgl64.Vec3
	PreviousServerPredictedMotion mgl64.Vec3
	JumpVelocity                  float64
	Gravity                       float64
	MoveForward                   float64
	MoveStrafe                    float64
	MovementSpeed                 float64
	TeleportOffset                uint8
	ySize                         float64
}

func (m *Movement) Execute(player utils.HasWorld) {
	s := m.Session
	if s.HasAnyFlag(FlagInUnloadedChunk, FlagInVoid, FlagImmobile, FlagFlying) || s.Gamemode != 0 || s.Ticks.Spawn < 10 {
		s.SetFlag(true, FlagOnGround)
		s.SetFlag(false, FlagCollidedVertically)
		m.PreviousServerPredictedMotion = m.Motion
		serverPredictedMotion := mgl64.Vec3{m.Motion.X(), (m.Motion.Y() - m.Gravity) * utils.GravityMultiplication * (0.6 * 0.91) * (0.6 * 0.91), m.Motion.Z()}
		m.ServerPredictedMotion = serverPredictedMotion
	} else {
		m.moveEntityWithHeading(player)
	}
}

func (m *Movement) moveEntityWithHeading(player utils.HasWorld) {
	s := m.Session
	entityData := s.GetEntityData()

	if s.Ticks.Motion == 0 {
		m.ServerPredictedMotion = omath.Vec32To64(m.ServerSentMotion)
		if s.HasFlag(FlagJumping) {
			m.ServerPredictedMotion = mgl64.Vec3{m.ServerPredictedMotion.X(), m.JumpVelocity, m.ServerPredictedMotion.Z()}
		}
		if s.HasFlag(FlagTeleporting) {
			m.Motion = omath.Vec32To64(m.ServerSentMotion)
			if s.HasFlag(FlagJumping) {
				m.Motion = mgl64.Vec3{m.Motion.X(), m.JumpVelocity, m.Motion.Z()}
			}
		}

		var1 := 0.91
		var var3 float64
		if s.HasFlag(FlagOnGround) {
			if s.HasFlag(FlagJumping) {
				m.jump()
			}
			if b, ok := player.Block(cube.PosFromVec3(omath.FloorVec64(entityData.LastPosition.Sub(mgl64.Vec3{0, 1})))).(block.Frictional); ok {
				var1 *= b.Friction()
			} else {
				var1 *= 0.6
			}
		} else {
			if s.HasFlag(FlagSprinting) {
				var3 = 0.026
			} else {
				var3 = 0.02
			}
		}
		// refer to https://media.discordapp.net/attachments/727159224320131133/868630080316928050/unknown.png
		var2 := math.Pow(0.546/var1, 3)
		if s.HasFlag(FlagOnGround) {
			var3 = m.MovementSpeed * var2
		}
		m.moveFlying(var3)
		if utils.BlockClimable(player.Block(cube.PosFromVec3(omath.FloorVec64(entityData.LastPosition)))) {
			f6 := 0.2
			yMotion := m.ServerPredictedMotion.Y()
			if yMotion < -0.2 {
				yMotion = -0.2
			}
			if s.HasFlag(FlagSneaking) && yMotion < 0 {
				yMotion = 0
			}
			m.ServerPredictedMotion = mgl64.Vec3{omath.ClampFloat(m.ServerPredictedMotion.Z(), -f6, f6), yMotion, omath.ClampFloat(m.ServerPredictedMotion.Z(), -f6, f6)}
		}
		cx, cz := m.moveEntity(player)
		if utils.BlockClimable(player.Block(cube.PosFromVec3(omath.FloorVec64(entityData.LastPosition)))) && s.HasFlag(FlagCollidedHorizontally) {
			m.ServerPredictedMotion = mgl64.Vec3{m.ServerPredictedMotion.X(), 0.2, m.ServerPredictedMotion.Z()}
		}
		m.PreviousServerPredictedMotion = m.ServerPredictedMotion

		//	TODO: Find a method that completes full compensation for stairs.
		//	These 4 lines are shitty hacks to compensate for an improper and incomplete stair prediction.
		//	In Minecraft bedrock, it seems that the player clips into the stairs, making the minecraft java
		//	movement code obsolete for this case.
		var hasStair bool
		for _, b := range utils.DefaultCheckBlockSettings(entityData.AABB.Grow(0.2), player).SearchAll() {
			if _, ok := b.Model().(model.Stair); ok {
				hasStair = true
				break
			}
		}

		if m.ySize > 1e-5 || hasStair && m.ServerPredictedMotion.Y() >= 0 && m.ServerPredictedMotion.Y() < 0.6 && m.Motion.Y() > -1e-6 && m.Motion.Y() < 1 {
			s.SetFlag(true, FlagOnGround)
			m.PreviousServerPredictedMotion = m.Motion
			m.ServerPredictedMotion = m.Motion
		}

		if s.HasFlag(FlagTeleporting) {
			m.TeleportOffset = 2
		}

		if m.TeleportOffset > 0 {
			yMotion := float64(m.ServerSentMotion.Y())
			if s.HasFlag(FlagJumping) {
				yMotion = m.JumpVelocity
				m.TeleportOffset = 1
			}
			s.SetFlag(true, FlagOnGround)
			m.TeleportOffset--
			m.ServerPredictedMotion = mgl64.Vec3{m.ServerPredictedMotion.X(), yMotion, m.ServerPredictedMotion.Z()}
		}

		x, y, z := m.ServerPredictedMotion.X(), m.ServerPredictedMotion.Y(), m.ServerPredictedMotion.Z()
		if cx {
			x = 0
		}
		if s.HasFlag(FlagCollidedVertically) {
			y = 0
		}
		if cz {
			z = 0
		}
		y = (y - m.Gravity) * utils.GravityMultiplication * var1 * var2
		m.ServerPredictedMotion = mgl64.Vec3{x, y, z}
	}
}

func (m *Movement) moveEntity(player utils.HasWorld) (bool, bool) {
	s := m.Session
	entityData := s.GetEntityData()
	dx, dy, dz := m.ServerPredictedMotion.X(), m.ServerPredictedMotion.Y(), m.ServerPredictedMotion.Z()
	movX, movY, movZ := dx, dy, dz
	// TODO: Prediction with collision on cobweb
	m.ySize *= 0.4

	oldBB := entityData.AABB.Translate(entityData.LastPosition)
	oldBB = oldBB.GrowVec3(mgl64.Vec3{-0.0025, 0, -0.0025})
	oldBBClone := oldBB

	if s.HasAllFlags(FlagOnGround, FlagSneaking) {
		mov := 0.05
		for ; dx != 0.0 && len(utils.DefaultCheckBlockSettings(oldBB.Translate(mgl64.Vec3{dx, -1, 0}), player).SearchAll()) == 0; movX = dx {
			if dx < mov && dx >= -mov {
				dx = 0
			} else if dx > 0 {
				dx -= mov
			} else {
				dx += mov
			}
		}
		for ; dz != 0.0 && len(utils.DefaultCheckBlockSettings(oldBB.Translate(mgl64.Vec3{0, -1, dz}), player).SearchAll()) == 0; movZ = dz {
			if dz < mov && dz >= -mov {
				dz = 0
			} else if dz > 0 {
				dz -= mov
			} else {
				dz += mov
			}
		}
	}

	list := utils.GetCollisionBBList(oldBB.Extend(mgl64.Vec3{dx, dy, dz}), player)
	for _, b := range list {
		dy = b.CalculateYOffset(oldBB, dy)
	}
	oldBB = oldBB.Translate(mgl64.Vec3{0, dy, 0})
	notFallingFlag := s.HasFlag(FlagOnGround) || (movY != dy && movY < 0)
	for _, b := range list {
		dx = b.CalculateXOffset(oldBB, dx)
	}
	oldBB = oldBB.Translate(mgl64.Vec3{dx, 0, 0})
	for _, b := range list {
		dz = b.CalculateZOffset(oldBB, dz)
	}
	oldBB = oldBB.Translate(mgl64.Vec3{0, 0, dz})

	if notFallingFlag && (movX != dx || movZ != dz) {
		cx, cz := dx, dz
		cy := dy
		dx, dy, dz = movX, utils.StepHeight, movZ

		oldBBClone2 := oldBB
		oldBB = oldBBClone

		list = utils.GetCollisionBBList(oldBB.Extend(mgl64.Vec3{dx, dy, dz}), player)
		for _, b := range list {
			dy = b.CalculateYOffset(oldBB, dy)
		}

		oldBB = oldBB.Translate(mgl64.Vec3{0, dy, 0})
		for _, b := range list {
			dx = b.CalculateYOffset(oldBB, dx)
		}

		oldBB = oldBB.Translate(mgl64.Vec3{dx, 0, 0})
		for _, b := range list {
			dz = b.CalculateYOffset(oldBB, dz)
		}

		oldBB = oldBB.Translate(mgl64.Vec3{0, 0, dz})

		reverseDY := -dy
		for _, b := range list {
			reverseDY = b.CalculateYOffset(oldBB, reverseDY)
		}
		dy = 0
		oldBB = oldBB.Translate(mgl64.Vec3{0, reverseDY, 0})

		if (math.Pow(cx, 2) + math.Pow(cz, 2)) >= (math.Pow(dx, 2) + math.Pow(dz, 2)) {
			dx, dy, dz = cx, cy, cz
			oldBB = oldBBClone2
		} else {
			m.ySize += dy
		}
	}

	s.SetFlag(movY != dy, FlagCollidedVertically)
	s.SetFlag(movX != dx || movZ != dz, FlagCollidedHorizontally)
	s.SetFlag(movY != dy && movY < 0, FlagOnGround)
	m.ServerPredictedMotion = mgl64.Vec3{dx, dy, dz}
	return movX != dx, movZ != dz
}

func (m *Movement) moveFlying(friction float64) {
	var1 := math.Pow(m.MoveForward, 2) + math.Pow(m.MoveStrafe, 2)
	if var1 >= 1e-4 {
		var1 = math.Sqrt(var1)
		if var1 < 1 {
			var1 = 1
		}
		var1 /= friction
		forward := m.MoveForward * var1
		strafe := m.MoveStrafe * var1
		yaw := m.Session.GetEntityData().Rotation.X()
		var2 := omath.MCSin(yaw * math.Pi / 180)
		var3 := omath.MCCos(yaw * math.Pi / 180)
		m.ServerPredictedMotion = mgl64.Vec3{m.ServerPredictedMotion.X() + (strafe*var3 - forward*var2), m.ServerPredictedMotion.Y(), m.ServerPredictedMotion.Z() + (forward*var3 + strafe*var2)}
	}
}

func (m *Movement) jump() {
	m.ServerPredictedMotion = mgl64.Vec3{m.ServerPredictedMotion.X(), m.JumpVelocity, m.ServerPredictedMotion.Z()}
	if m.Session.HasFlag(FlagSprinting) {
		f := m.Session.GetEntityData().Rotation.X() * 0.017453292
		m.ServerPredictedMotion = mgl64.Vec3{m.ServerPredictedMotion.X() - omath.MCSin(f)*0.2, m.ServerPredictedMotion.Y(), m.ServerPredictedMotion.Z() + omath.MCCos(f)*0.2}
	}
}
