package player

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"math"
)

// tickMovement ticks the player's movement server-side and ensures it matches up with the client.
func (p *Player) tickMovement() {
	if !p.ready || p.inVoid || p.immobile || p.flying || p.gameMode > 0 || p.spawnTicks < 10 {
		p.onGround = true
		p.collidedVertically = true
		p.previousServerPredictedMotion = p.motion
		p.serverPredictedMotion = mgl64.Vec3{
			p.motion.X() * (0.6 * 0.91),
			(p.motion.Y() - p.gravity) * game.GravityMultiplier,
			p.motion.Z() * (0.6 * 0.91),
		}
		return
	}
	p.moveWithHeading()
}

// moveWithHeading moves the player with the current heading.
func (p *Player) moveWithHeading() {
	w := p.World()
	e := p.Entity()

	if p.motionTicks == 0 {
		p.serverPredictedMotion = game.Vec32To64(p.serverSentMotion)
		if p.jumping {
			p.serverPredictedMotion[1] = p.jumpVelocity
		}
		if p.teleporting {
			p.motion = game.Vec32To64(p.serverSentMotion)
			if p.jumping {
				p.motion[1] = p.jumpVelocity
			}
		}
	}

	var1 := 0.98
	var var3 float64
	if p.onGround {
		if p.jumping {
			p.jump()
		}
		if b, ok := w.Block(cube.PosFromVec3(game.FloorVec64(e.LastPosition().Sub(mgl64.Vec3{0, 1})))).(block.Frictional); ok {
			var1 *= b.Friction()
		} else {
			var1 *= 0.6
		}
	} else {
		if p.sprinting {
			var3 = 0.026
		} else {
			var3 = 0.02
		}
	}

	var2 := math.Pow(0.546/var1, 3)
	if p.onGround {
		var3 = p.speed * var2
	}

	p.moveFlying(var3)

	if utils.BlockClimbable(w.Block(cube.PosFromVec3(game.FloorVec64(e.LastPosition())))) {
		f6 := 0.2
		yMotion := p.serverPredictedMotion.Y()
		if yMotion < -0.2 {
			yMotion = -0.2
		}
		if p.sneaking && yMotion < 0 {
			yMotion = 0
		}
		p.serverPredictedMotion = mgl64.Vec3{
			game.ClampFloat(p.serverPredictedMotion.Z(), -f6, f6),
			yMotion,
			game.ClampFloat(p.serverPredictedMotion.Z(), -f6, f6),
		}
	}

	cx, cz := p.move()
	if utils.BlockClimbable(w.Block(cube.PosFromVec3(game.FloorVec64(e.LastPosition())))) && p.collidedHorizontally {
		p.serverPredictedMotion[1] = 0.2
	}
	p.previousServerPredictedMotion = p.serverPredictedMotion

	// TODO: Find a method that completes full compensation for stairs.
	//  These 7 lines are shitty hacks to compensate for an improper and incomplete stair prediction.
	//  In Minecraft bedrock, it seems that the player clips into the stairs, making the minecraft java
	//  movement code obsolete for this case.
	var hasStair bool
	for _, b := range utils.BlocksNearby(p.Position(), e.AABB().Grow(0.2), w) {
		if _, ok := b.Model().(model.Stair); ok {
			hasStair = true
			break
		}
	}

	if p.ySize > 1e-5 || hasStair && p.serverPredictedMotion.Y() >= 0 && p.serverPredictedMotion.Y() < 0.6 && p.motion.Y() > -1e-6 && p.motion.Y() < 1 {
		p.onGround = true
		p.previousServerPredictedMotion = p.motion
		p.serverPredictedMotion = p.motion
	}

	if p.teleporting {
		p.teleportOffset = 2
	}

	if p.teleportOffset > 0 {
		yMotion := float64(p.serverSentMotion.Y())
		if p.jumping {
			yMotion = p.jumpVelocity
			p.teleportOffset = 1
		}
		p.onGround = true
		p.teleportOffset--
		p.serverPredictedMotion[1] = yMotion
	}

	x, y, z := p.serverPredictedMotion.X(), p.serverPredictedMotion.Y(), p.serverPredictedMotion.Z()
	if cx {
		x = 0
	}
	if p.collidedVertically {
		y = 0
	}
	if cz {
		z = 0
	}

	p.serverPredictedMotion = mgl64.Vec3{
		x * var1,
		(y - p.gravity) * game.GravityMultiplier,
		z * var1,
	}
}

// moveFlying moves the player in air.
func (p *Player) moveFlying(friction float64) {
	var1 := math.Pow(p.moveForward, 2) + math.Pow(p.moveStrafe, 2)
	if var1 >= 1e-4 {
		var1 = math.Sqrt(var1)
		if var1 < 1 {
			var1 = 1
		}
		var1 = friction / var1
		forward := p.moveForward * var1
		strafe := p.moveStrafe * var1
		yaw := p.Rotation().Z()
		var2 := game.MCSin(yaw * math.Pi / 180)
		var3 := game.MCCos(yaw * math.Pi / 180)
		p.serverPredictedMotion[0] += strafe*var3 - forward*var2
		p.serverPredictedMotion[2] += forward*var3 + strafe*var2
	}
}

// move uses the server predicted motion to move the player to the expected position.
func (p *Player) move() (bool, bool) {
	dx, dy, dz := p.serverPredictedMotion.X(), p.serverPredictedMotion.Y(), p.serverPredictedMotion.Z()
	movX, movY, movZ := dx, dy, dz

	oldPos := p.Entity().LastPosition()
	aabb := p.AABB()
	w := p.World()

	// TODO: Prediction with collision on cobweb
	p.ySize *= 0.4

	if p.onGround && p.sneaking {
		clone := aabb
		mov := 0.05
		for ; dx != 0.0 && len(utils.BlocksNearby(oldPos, clone.Translate(mgl64.Vec3{dx, -1, 0}), w)) == 0; movX = dx {
			if dx < mov && dx >= -mov {
				dx = 0
			} else if dx > 0 {
				dx -= mov
			} else {
				dx += mov
			}
		}
		for ; dz != 0.0 && len(utils.BlocksNearby(oldPos, clone.Translate(mgl64.Vec3{0, -1, dz}), w)) == 0; movZ = dz {
			if dz < mov && dz >= -mov {
				dz = 0
			} else if dz > 0 {
				dz -= mov
			} else {
				dz += mov
			}
		}
	}

	clone := aabb
	list := utils.CollidingBlocks(clone.Extend(mgl64.Vec3{dx, dy, dz}), p.Position(), w)
	for _, b := range list {
		dy = clone.CalculateYOffset(b, dy)
	}
	clone = clone.Translate(mgl64.Vec3{0, dy, 0})
	for _, b := range list {
		dx = clone.CalculateXOffset(b, dx)
	}
	clone = clone.Translate(mgl64.Vec3{dx, 0, 0})
	for _, b := range list {
		dz = clone.CalculateZOffset(b, dz)
	}
	clone = clone.Translate(mgl64.Vec3{0, 0, dz})

	if (p.onGround || (movY != dy && movY < 0)) && (movX != dx || movZ != dz) {
		cx, cz := dx, dz
		cy := dy
		dx, dy, dz = movX, game.StepHeight, movZ

		clone = aabb
		list = utils.CollidingBlocks(clone.Extend(mgl64.Vec3{dx, dy, dz}), p.Position(), w)
		for _, b := range list {
			dy = clone.CalculateYOffset(b, dy)
		}
		clone = clone.Translate(mgl64.Vec3{0, dy, 0})
		for _, b := range list {
			dx = clone.CalculateXOffset(b, dx)
		}
		clone = clone.Translate(mgl64.Vec3{dx, 0, 0})
		for _, b := range list {
			dz = clone.CalculateZOffset(b, dz)
		}
		clone = clone.Translate(mgl64.Vec3{0, 0, dz})

		dy = 0
		reverseDY := -dy
		for _, b := range list {
			reverseDY = clone.CalculateYOffset(b, reverseDY)
		}
		clone = clone.Translate(mgl64.Vec3{0, reverseDY, 0})

		if (math.Pow(cx, 2) + math.Pow(cz, 2)) >= (math.Pow(dx, 2) + math.Pow(dz, 2)) {
			dx, dy, dz = cx, cy, cz
		}
	}

	p.collidedVertically = movY != dy
	p.collidedHorizontally = movX != dx || movZ != dz
	p.onGround = movY != dy && movY < 0

	p.serverPredictedMotion = mgl64.Vec3{dx, dy, dz}
	return movX != dx, movZ != dz
}

// jump moves the player up by their jump velocity.
func (p *Player) jump() {
	p.serverPredictedMotion = mgl64.Vec3{
		p.serverPredictedMotion.X(),
		p.jumpVelocity,
		p.serverPredictedMotion.Z(),
	}
	if p.Sprinting() {
		f := p.Rotation().Z() * 0.017453292
		p.serverPredictedMotion = mgl64.Vec3{
			p.serverPredictedMotion.X() - game.MCSin(f)*0.2,
			p.serverPredictedMotion.Y(),
			p.serverPredictedMotion.Z() + game.MCCos(f)*0.2,
		}
	}
}
