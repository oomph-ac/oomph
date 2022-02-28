package player

import (
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"math"
)

// tickMovement ticks the player's movement server-side and ensures it matches up with the client.
func (p *Player) tickMovement() {
	if p.inUnloadedChunk || p.inVoid || p.immobile || p.flying || p.gameMode > 0 || p.spawnTicks < 10 {
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
	// TODO
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
func (p *Player) move() {
	// TODO
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
