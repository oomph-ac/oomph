package player

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/ethaniccc/float32-cube/cube"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// updateMovementState updates the movement state of the player - this function will check if any exemptions need to be made.
// If no exemptions are needed, then this function will proceed to calculate the expected movement and position of the player this simulation frame.
func (p *Player) updateMovementState() bool {
	var exempt bool
	if p.inLoadedChunkTicks < 100 || !p.ready || p.mInfo.InVoid || p.mInfo.Flying || p.mInfo.NoClip {
		p.mInfo.OnGround = true
		p.mInfo.VerticallyCollided = true
		p.mInfo.ServerPosition = p.Position()
		p.mInfo.OldServerMovement = p.mInfo.ClientMovement
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
		p.mInfo.CanExempt = true
		exempt = true
	} else {
		p.mInfo.InUnsupportedRewindScenario = false
		exempt = p.mInfo.CanExempt
		p.aiStep()
		p.mInfo.CanExempt = false
	}

	p.mInfo.UpdateTickStatus()
	return !exempt
}

// validateMovement validates the movement of the player. If the position or the velocity of the player is offset by a certain amount, the player's movement will be corrected.
// If the player's position is within a certain range of the server's predicted position, then the server's position is set to the client's
func (p *Player) validateMovement() {
	if p.movementMode != utils.ModeFullAuthoritative {
		return
	}

	diff := p.mInfo.ServerPosition.Sub(p.Position())
	if diff.LenSqr() < 0.09 {
		return
	}

	p.correctMovement()
}

// correctMovement sends a movement correction to the player. Exemptions can be made to prevent sending corrections, such as if
// the player has not received a correction yet, if the player is teleporting, or if the player is in an unsupported rewind scenario
// (determined by the people that made the rewind system) - in which case movement corrections will not work properly.
func (p *Player) correctMovement() {
	// Do not correct player movement if the movement mode is not fully server authoritative because it can lead to issues.
	if p.movementMode != utils.ModeFullAuthoritative {
		return
	}

	if p.mInfo.CanExempt || p.mInfo.Teleporting || p.mInfo.InUnsupportedRewindScenario {
		return
	}

	pos, delta := p.mInfo.ServerPosition, p.mInfo.ServerMovement

	// Send block updates for blocks around the player - to make sure that the world state
	// on the client is the same as the server's.
	if p.mInfo.RefreshBlockTicks >= 30 {
		for bpos, b := range p.GetNearbyBlocks(p.AABB().Translate(p.mInfo.ServerPosition)) {
			p.conn.WritePacket(&packet.UpdateBlock{
				Position:          protocol.BlockPos{int32(bpos.X()), int32(bpos.Y()), int32(bpos.Z())},
				NewBlockRuntimeID: world.BlockRuntimeID(b),
				Flags:             packet.BlockUpdateNeighbours,
				Layer:             0,
			})
		}
		p.mInfo.RefreshBlockTicks = 0
	}

	// This will send the most recent actor data to the client to ensure that all
	// actor data is the same on the client and server (sprinting, sneaking, swimming, etc.)
	if p.lastSentActorData != nil {
		p.lastSentActorData.Tick = 0
		p.conn.WritePacket(p.lastSentActorData)
	}

	// This will send the most recent player attributes to the client to ensure
	// all attributes are the same on the client and server (health, speed, etc.)
	if p.lastSentAttributes != nil {
		p.lastSentAttributes.Tick = 0
		p.conn.WritePacket(p.lastSentAttributes)
	}

	// This packet will correct the player to the server's predicted position.
	p.conn.WritePacket(&packet.CorrectPlayerMovePrediction{
		Position: pos.Add(mgl32.Vec3{0, 1.62 + 1e-3}),
		Delta:    delta,
		OnGround: p.mInfo.OnGround,
		Tick:     p.ClientFrame(),
	})
}

// aiStep starts the movement simulation of the player.
func (p *Player) aiStep() {
	if p.mInfo.Immobile || !p.inLoadedChunk {
		p.mInfo.ForwardImpulse = 0.0
		p.mInfo.LeftImpulse = 0.0
		p.mInfo.Jumping = false

		p.mInfo.ServerMovement = mgl32.Vec3{}
	}

	if mgl32.Abs(p.mInfo.ServerMovement[0]) < 0.003 {
		p.mInfo.ServerMovement[0] = 0
	}

	if mgl32.Abs(p.mInfo.ServerMovement[1]) < 0.003 {
		p.mInfo.ServerMovement[1] = 0
	}

	if mgl32.Abs(p.mInfo.ServerMovement[2]) < 0.003 {
		p.mInfo.ServerMovement[2] = 0
	}

	p.mInfo.FlyingSpeed = 0.02
	if p.mInfo.Sprinting {
		p.mInfo.FlyingSpeed += 0.006
	}

	if p.mInfo.MotionTicks == 0 {
		p.mInfo.ServerMovement = p.mInfo.ServerSentMovement
	}

	if !p.mInfo.JumpDown {
		p.mInfo.JumpCooldownTicks = 0
	}

	if p.mInfo.Jumping && p.mInfo.OnGround && p.mInfo.JumpCooldownTicks <= 0 {
		p.simulateJump()
		p.mInfo.JumpCooldownTicks = 10
	}

	p.travel()
	p.checkUnsupportedMovementScenarios()
}

// travel continues the player's movement simulation.
func (p *Player) travel() {
	blockFriction := float32(0.91)
	if p.mInfo.OnGround {
		if b, ok := p.Block(cube.PosFromVec3(p.mInfo.ServerPosition).Side(cube.FaceDown)).(block.Frictional); ok {
			blockFriction *= float32(b.Friction())
		} else {
			blockFriction *= 0.6
		}
	}

	v3 := p.mInfo.getFrictionInfluencedSpeed(blockFriction)
	p.simulateAddedMovementForce(v3)

	climb := utils.BlockClimbable(p.Block(cube.PosFromVec3(p.mInfo.ServerPosition)))
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

	p.maybeBackOffFromEdge()
	oldMov := p.mInfo.ServerMovement
	p.collide(p.mInfo.ServerMovement)
	p.mInfo.ServerPosition = p.mInfo.ServerPosition.Add(p.mInfo.ServerMovement)
	p.checkCollisions(oldMov)

	if climb && (p.mInfo.HorizontallyCollided || p.mInfo.JumpDown) {
		p.mInfo.ServerMovement[1] = 0.3
	}

	p.checkUnsupportedMovementScenarios()
	p.mInfo.OldServerMovement = p.mInfo.ServerMovement

	if !p.mInfo.InUnsupportedRewindScenario {
		p.simulateGravity()
		p.simulateHorizontalFriction(blockFriction)
	}
}

// simulateAddedMovementForce simulates the additional movement force created by the player's mf/ms and rotation values
func (p *Player) simulateAddedMovementForce(f float32) {
	v := math32.Pow(p.mInfo.ForwardImpulse, 2) + math32.Pow(p.mInfo.LeftImpulse, 2)
	if v < 1e-4 {
		return
	}

	v = math32.Sqrt(v)
	if v < 1 {
		v = 1
	}
	v = f / v
	mf, ms := p.mInfo.ForwardImpulse*v, p.mInfo.LeftImpulse*v
	v2, v3 := game.MCSin(p.entity.Rotation().Z()*math32.Pi/180), game.MCCos(p.entity.Rotation().Z()*math32.Pi/180)
	p.mInfo.ServerMovement[0] += ms*v3 - mf*v2
	p.mInfo.ServerMovement[2] += ms*v2 + mf*v3
}

// maybeBackOffFromEdge simulates the movement scenarios where a player is at the edge of a block.
// The weird function name comes from MCP, but what else would it be named I guess lol
func (p *Player) maybeBackOffFromEdge() {
	if !p.mInfo.Sneaking || p.mInfo.ServerMovement[1] > 0 {
		return
	}

	v := p.mInfo.ServerMovement
	if v[1] > 0 {
		return
	}

	bb := p.AABB().Translate(p.mInfo.ServerPosition)
	d0, d1, d2 := v[0], v[2], float32(0.05)

	for d0 != 0 && len(p.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{d0, -game.StepHeight, 0}))) == 0 {
		if d0 < d2 && d0 >= -d2 {
			d0 = 0
		} else if d0 > 0 {
			d0 -= d2
		} else {
			d0 += d2
		}
	}

	for d1 != 0 && len(p.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -game.StepHeight, d1}))) == 0 {
		if d1 < d2 && d1 >= -d2 {
			d1 = 0
		} else if d1 > 0 {
			d1 -= d2
		} else {
			d1 += d2
		}
	}

	for d0 != 0 && d1 != 0 && len(p.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{d0, -game.StepHeight, d1}))) == 0 {
		if d0 < d2 && d0 >= -d2 {
			d0 = 0
		} else if d0 > 0 {
			d0 -= d2
		} else {
			d0 += d2
		}

		if d1 < d2 && d1 >= -d2 {
			d1 = 0
		} else if d1 > 0 {
			d1 -= d2
		} else {
			d1 += d2
		}
	}

	p.mInfo.ServerMovement = mgl32.Vec3{d0, v[1], d1}
}

// collide simulates the player's collisions with blocks
func (p *Player) collide(v mgl32.Vec3) {
	bb := p.AABB().Translate(p.mInfo.ServerPosition)
	l := p.GetNearbyBBoxes(bb.Extend(v))

	v2 := v
	if p.mInfo.ServerMovement.LenSqr() > 0.0 {
		v2 = p.collideWithBlocks(v, bb, l)
	}

	f := v[0] != v2[0]                         // Checks if the old X velocity differs from the new X velocity.
	f1 := v[1] != v2[1]                        // Checks if the old Y velocity differs from the new Y velocity.
	f2 := v[2] != v2[2]                        // Checks if the old Z velocity differs from the new Z velocity.
	f3 := p.mInfo.OnGround || f1 && v[1] < 0.0 // Checks if the player is currently in an on-ground state.
	if f3 && (f || f2) {
		// MCP 1.19 shit here we go!
		v31 := p.collideWithBlocks(mgl32.Vec3{v[0], game.StepHeight, v[2]}, bb, l)
		v32 := p.collideWithBlocks(mgl32.Vec3{0, v[1], 0}, bb.Extend(mgl32.Vec3{v[0], 0, v[2]}), l)

		if v32[1] < game.StepHeight {
			v33 := p.collideWithBlocks(mgl32.Vec3{v[0], 0, v[2]}, bb.Translate(v32), l).Add(v32)
			if game.Vec3HzDistSqr(v33) > game.Vec3HzDistSqr(v31) {
				v31 = v33
			}
		}

		if game.Vec3HzDistSqr(v31) > game.Vec3HzDistSqr(v2) {
			p.mInfo.ServerMovement = v31.Add(p.collideWithBlocks(mgl32.Vec3{0, -v31[1] + v[1], 0}, bb.Translate(v31), l))
			return
		}
	}

	p.mInfo.ServerMovement = v2
}

// collideWithBlocks simulates the player's collisions with blocks
func (p *Player) collideWithBlocks(vel mgl32.Vec3, bb cube.BBox, list []cube.BBox) mgl32.Vec3 {
	if len(list) == 0 {
		return vel
	}

	d0, d1, d2 := vel[0], vel[1], vel[2]
	if d1 != 0 {
		bb, d1 = utils.DoBoxCollision(utils.CollisionY, bb, list, d1)
	}

	flag := math32.Abs(d0) < math32.Abs(d2)
	if flag && d2 != 0 {
		bb, d2 = utils.DoBoxCollision(utils.CollisionZ, bb, list, d2)
	}

	if d0 != 0 {
		bb, d0 = utils.DoBoxCollision(utils.CollisionX, bb, list, d0)
	}

	if !flag && d2 != 0 {
		bb, d2 = utils.DoBoxCollision(utils.CollisionZ, bb, list, d2)
	}

	return mgl32.Vec3{d0, d1, d2}
}

// simulateGravity simulates the gravity of the player
func (p *Player) simulateGravity() {
	p.mInfo.ServerMovement[1] -= p.mInfo.Gravity
	p.mInfo.ServerMovement[1] *= 0.98
}

// simulateHorizontalFriction simulates the horizontal friction of the player
func (p *Player) simulateHorizontalFriction(friction float32) {
	p.mInfo.ServerMovement[0] *= friction
	p.mInfo.ServerMovement[2] *= friction
}

// simulateJump simulates the jump movement of the player
func (p *Player) simulateJump() {
	p.mInfo.ServerMovement[1] = p.mInfo.JumpVelocity
	if !p.mInfo.Sprinting {
		return
	}

	force := p.entity.Rotation().Z() * 0.017453292
	p.mInfo.ServerMovement[0] -= game.MCSin(force) * 0.2
	p.mInfo.ServerMovement[2] += game.MCCos(force) * 0.2
}

// setMovementToClient sets the server's velocity and position to the client's
func (p *Player) setMovementToClient() {
	p.mInfo.ServerPosition = p.Position()
	p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
}

// checkCollisions compares the old and new velocities to check if there were any collisions made in p.collide()
func (p *Player) checkCollisions(old mgl32.Vec3) {
	p.mInfo.XCollision = !mgl32.FloatEqualThreshold(old[0], p.mInfo.ServerMovement[0], 1e-5)
	p.mInfo.ZCollision = !mgl32.FloatEqualThreshold(old[2], p.mInfo.ServerMovement[2], 1e-5)

	p.mInfo.HorizontallyCollided = p.mInfo.XCollision || p.mInfo.ZCollision
	p.mInfo.VerticallyCollided = old[1] != p.mInfo.ServerMovement[1]
	p.mInfo.OnGround = p.mInfo.VerticallyCollided && old[1] < 0.0

	if p.mInfo.VerticallyCollided {
		p.mInfo.ServerMovement[1] = 0
	}

	if p.mInfo.XCollision {
		p.mInfo.ServerMovement[0] = 0
	}

	if p.mInfo.ZCollision {
		p.mInfo.ServerMovement[2] = 0
	}
}

// checkUnsupportedMovementScenarios checks if the player is in an unsupported movement scenario
func (p *Player) checkUnsupportedMovementScenarios() {
	bb := p.AABB().Translate(p.mInfo.ServerPosition).Grow(-1e-6)
	//boxes = p.GetNearbyBBoxes(bb)
	blocks := p.GetNearbyBlocks(bb)

	/* The following checks below determine wether or not the player is in an unspported rewind scenario.
	What this means is that the movement corrections on the client won't work properly and the player will
	essentially be jerked around indefinently, and therefore, corrections should not be done if these conditions
	are met. */

	// This check determines if the player is inside any blocks
	/* if cube.AnyIntersections(boxes, bb) && !p.mInfo.HorizontallyCollided && !p.mInfo.VerticallyCollided {
		p.mInfo.InUnsupportedRewindScenario = true
	} */

	// This check determines if the player is near liquids
	for _, bl := range blocks {
		_, ok := bl.(world.Liquid)
		if ok {
			p.mInfo.InUnsupportedRewindScenario = true
			break
		}
	}

	if p.mInfo.InUnsupportedRewindScenario {
		p.setMovementToClient()
	}
}

type MovementInfo struct {
	CanExempt bool

	InUnsupportedRewindScenario bool

	ForwardImpulse, LeftImpulse float32
	JumpVelocity                float32
	FlyingSpeed                 float32
	Gravity                     float32
	Speed                       float32

	MotionTicks       uint32
	RefreshBlockTicks uint32
	JumpCooldownTicks int32

	Sneaking, SneakDown   bool
	Jumping, JumpDown     bool
	Sprinting, SprintDown bool
	Teleporting           bool
	Immobile              bool
	CanFly, Flying        bool
	NoClip                bool

	IsCollided, VerticallyCollided, HorizontallyCollided bool
	XCollision, ZCollision                               bool
	OnGround                                             bool
	InVoid                                               bool

	ClientPredictedMovement, ClientMovement mgl32.Vec3
	ServerSentMovement                      mgl32.Vec3
	ServerMovement, OldServerMovement       mgl32.Vec3
	ServerPosition                          mgl32.Vec3

	LastUsedInput *packet.PlayerAuthInput
}

func (m *MovementInfo) UpdateServerSentVelocity(velo mgl32.Vec3) {
	m.ServerSentMovement = velo
	m.MotionTicks = 0
}

func (m *MovementInfo) UpdateTickStatus() {
	m.MotionTicks++
	m.RefreshBlockTicks++

	m.JumpCooldownTicks--
}

func (m *MovementInfo) getFrictionInfluencedSpeed(f float32) float32 {
	if m.OnGround {
		return m.Speed * math32.Pow(0.546/f, 3)
	}

	return m.FlyingSpeed
}
