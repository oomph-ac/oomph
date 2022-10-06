package player

import (
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// updateMovementState updates the movement state of the player - this function will check if any exemptions need to be made.
// If no exemptions are needed, then this function will proceed to calculate the expected movement and position of the player this simulation frame.
func (p *Player) updateMovementState() bool {
	var exempt bool
	if p.inLoadedChunkTicks < 100 || !p.ready || p.mInfo.InVoid || p.mInfo.Flying || (p.gameMode != packet.GameTypeSurvival && p.gameMode != packet.GameTypeAdventure) || p.mInfo.NoClip {
		p.mInfo.OnGround = true
		p.mInfo.VerticallyCollided = true
		p.mInfo.ServerPosition = p.Position()
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
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

// validateMovement validates the movement of the player. If the position or the velocity of the player is offset by a certain amount, the player's movement will be corrected.
// If the player's position is within a certain range of the server's predicted position, then the server's position is set to the client's
func (p *Player) validateMovement() {
	if p.movementMode != utils.ModeFullAuthoritative {
		return
	}

	posError, velError := p.mInfo.ServerPosition.Sub(p.Position()).LenSqr(), p.mInfo.ServerMovement.Sub(p.mInfo.ClientPredictedMovement).LenSqr()

	if posError > 0.04 || velError > 0.25 {
		p.correctMovement()
		return
	}

	if velError <= 1e-6 {
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
	}

	if posError <= 1e-6 {
		p.mInfo.ServerPosition = p.Position()
	}
}

// correctMovement sends a movement correction to the player. Exemptions can be made to prevent sending corrections, such as if
// the player has not recieved a correction yet, if the player is teleporting, or if the player is in an unsupported rewind scenario
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
	for bpos, b := range p.GetNearbyBlocks(p.AABB().Translate(p.mInfo.ServerPosition)) {
		layer := uint32(0)
		if _, ok := b.(world.Liquid); ok {
			layer = 1
		}

		p.conn.WritePacket(&packet.UpdateBlock{
			Position:          protocol.BlockPos{int32(bpos.X()), int32(bpos.Y()), int32(bpos.Z())},
			NewBlockRuntimeID: world.BlockRuntimeID(b),
			Flags:             packet.BlockUpdateNeighbours,
			Layer:             layer,
		})
	}

	// This will send the most recent actor data to the client to ensure that all
	// actor data is the same on the client and server (sprinting, sneaking, swimming, etc.)
	if p.lastSentActorData != nil {
		p.lastSentActorData.Tick = p.ClientFrame()
		p.conn.WritePacket(p.lastSentActorData)
	}

	// This will send the most recent player attributes to the client to ensure
	// all attributes are the same on the client and server (health, speed, etc.)
	if p.lastSentAttributes != nil {
		p.lastSentAttributes.Tick = p.ClientFrame()
		p.conn.WritePacket(p.lastSentAttributes)
	}

	// This packet will correct the player to the server's predicted position.
	p.conn.WritePacket(&packet.CorrectPlayerMovePrediction{
		Position: game.Vec64To32(pos.Add(mgl64.Vec3{0, 1.62 + 1e-3})),
		Delta:    game.Vec64To32(delta),
		OnGround: p.mInfo.OnGround,
		Tick:     p.ClientFrame(),
	})
}

// processInput processes the input packet sent by the client to the server. This also updates some of the movement states such as
// if the player is sprinting, jumping, or in a loaded chunk.
func (p *Player) processInput(pk *packet.PlayerAuthInput) {
	p.miMu.Lock()
	defer p.miMu.Unlock()

	p.inputMode = pk.InputMode
	p.Move(pk)

	if p.movementMode == utils.ModeClientAuthoritative {
		p.mInfo.ServerPosition = p.Position()
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
		return
	}

	p.inLoadedChunk = p.ChunkExists(protocol.ChunkPos{
		int32(math.Floor(p.mInfo.ServerPosition[0])) >> 4,
		int32(math.Floor(p.mInfo.ServerPosition[2])) >> 4,
	})

	if p.inLoadedChunk {
		p.inLoadedChunkTicks++
	} else {
		p.inLoadedChunkTicks = 0
	}

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
	p.mInfo.InVoid = p.Position().Y() < -128

	p.mInfo.JumpVelocity = game.DefaultJumpMotion
	p.mInfo.Gravity = game.NormalGravity

	p.tickEffects()

	if utils.HasFlag(pk.InputData, packet.InputFlagPerformItemInteraction) && pk.ItemInteractionData.ActionType == protocol.UseItemActionBreakBlock {
		b, _ := world.BlockByRuntimeID(air)
		p.SetBlock(cube.Pos{
			int(pk.ItemInteractionData.BlockPosition[0]),
			int(pk.ItemInteractionData.BlockPosition[1]),
			int(pk.ItemInteractionData.BlockPosition[2]),
		}, b)
	}

	if p.updateMovementState() {
		p.validateMovement()
	}

	if p.movementMode == utils.ModeFullAuthoritative {
		pk.Position = game.Vec64To32(p.mInfo.ServerPosition.Add(mgl64.Vec3{0, 1.62}))
	}

	p.mInfo.Teleporting = false
}

// calculateExpectedMovement calculates the expected movement of the player for it's simulation frame.
func (p *Player) calculateExpectedMovement() {
	// If the player is immobile, then they should not be able to move at all. Instead of calculating all
	// of the collisions that won't be nessesary anyway as the player will be "stuck", we can just assume that their
	// position will be the same, and that their movement will be a zero vector.
	if p.mInfo.Immobile {
		p.mInfo.ServerMovement = mgl64.Vec3{0, 0, 0}
		return
	}

	if p.mInfo.MotionTicks == 0 {
		p.mInfo.ServerMovement = p.mInfo.ServerSentMovement
	}

	if p.mInfo.Jumping && p.mInfo.OnGround {
		p.simulateJump()
	}

	v1 := 0.91
	if p.mInfo.OnGround {
		if b, ok := p.Block(cube.PosFromVec3(p.mInfo.ServerPosition).Side(cube.FaceDown)).(block.Frictional); ok {
			v1 *= b.Friction()
		} else {
			v1 *= 0.6
		}
	}

	var v3 float64
	if p.mInfo.OnGround {
		v3 = p.mInfo.Speed * math.Pow((0.91*0.6)/v1, 3)
	} else if p.mInfo.Sprinting {
		v3 = 0.026 // 0.02 + (0.02 * 0.3)
	} else {
		v3 = 0.02
	}

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

	hc := p.mInfo.HorizontallyCollided
	p.simulateCollisions()

	if climb && (p.mInfo.HorizontallyCollided || p.mInfo.JumpDown) {
		p.mInfo.ServerMovement[1] = 0.3
	}

	if mgl64.Abs(p.mInfo.ServerMovement[0]) < 1e-10 {
		p.mInfo.ServerMovement[0] = 0
	}
	if mgl64.Abs(p.mInfo.ServerMovement[1]) < 1e-10 {
		p.mInfo.ServerMovement[1] = 0
	}
	if mgl64.Abs(p.mInfo.ServerMovement[2]) < 1e-10 {
		p.mInfo.ServerMovement[2] = 0
	}

	// After colliding with a block horizontally, the client stops sprinting. However, there seems to be a desync,
	// where the client will collide with a block horizontally and not send it's status for itself to stop sprinting.
	// This behavior is also noticable in a BDS server with movement corrections enabled.
	if hc && !p.mInfo.SprintDown && p.mInfo.MoveForward <= 0 {
		p.mInfo.Sprinting = false
	}

	p.mInfo.OldServerMovement = p.mInfo.ServerMovement

	p.simulateGravity()
	p.simulateHorizontalFriction(v1)
}

// simulateAddedMovementForce simulates the additional movement force created by the player's mf/ms and rotation values
func (p *Player) simulateAddedMovementForce(f float64) {
	v := math.Pow(p.mInfo.MoveForward, 2) + math.Pow(p.mInfo.MoveStrafe, 2)
	if v < 1e-4 {
		return
	}

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

// simulateCollisions simulates the player's collisions with blocks
func (p *Player) simulateCollisions() {
	p.mInfo.StepLenience *= 0.4
	if p.mInfo.StepLenience <= 1e-4 {
		p.mInfo.StepLenience = 0
	}

	vel := p.mInfo.ServerMovement
	deltaX, deltaY, deltaZ := vel[0], vel[1], vel[2]

	moveBB := p.AABB().Translate(p.mInfo.ServerPosition).Grow(-5e-4)
	cloneBB := moveBB
	boxes := p.GetNearbyBBoxes(cloneBB.Extend(vel))

	if p.mInfo.OnGround && p.mInfo.Sneaking {
		mov := 0.05

		for ; deltaX != 0.0 && len(p.GetNearbyBBoxes(moveBB.Translate(mgl64.Vec3{deltaX, -1, 0}))) == 0; vel[0] = deltaX {
			if deltaX < mov && deltaX >= -mov {
				deltaX = 0
			} else if deltaX > 0 {
				deltaX -= mov
			} else {
				deltaX += mov
			}
		}

		for ; deltaZ != 0.0 && len(p.GetNearbyBBoxes(moveBB.Translate(mgl64.Vec3{0, -1, deltaZ}))) == 0; vel[2] = deltaZ {
			if deltaZ < mov && deltaZ >= -mov {
				deltaZ = 0
			} else if deltaZ > 0 {
				deltaZ -= mov
			} else {
				deltaZ += mov
			}
		}

		for ; deltaX != 0 && deltaZ != 0 && len(p.GetNearbyBBoxes(moveBB.Translate(mgl64.Vec3{deltaX, -1, deltaZ}))) == 0; vel[2] = deltaZ {
			if deltaX < mov && deltaX >= -mov {
				deltaX = 0
			} else if deltaX > 0 {
				deltaX -= mov
			} else {
				deltaX += mov
			}
			vel[0] = deltaX

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
	for _, blockBBox := range boxes {
		deltaY = moveBB.YOffset(blockBBox, deltaY)
	}
	moveBB = moveBB.Translate(mgl64.Vec3{0, deltaY})

	flag := p.mInfo.OnGround || (vel[1] != deltaY && vel[1] < 0)

	// Afterward, check for collisions on the X and Z axis
	for _, blockBBox := range boxes {
		deltaX = moveBB.XOffset(blockBBox, deltaX)
	}
	moveBB = moveBB.Translate(mgl64.Vec3{deltaX})
	for _, blockBBox := range boxes {
		deltaZ = moveBB.ZOffset(blockBBox, deltaZ)
	}
	moveBB = moveBB.Translate(mgl64.Vec3{0, 0, deltaZ})

	if flag && ((vel[0] != deltaX) || (vel[2] != deltaZ)) {
		cx, cy, cz := deltaX, deltaY, deltaZ
		deltaX, deltaY, deltaZ = vel[0], game.StepHeight, vel[2]

		stepBB := p.AABB().Translate(p.mInfo.ServerPosition)
		cloneBB = stepBB
		boxes = p.GetNearbyBBoxes(cloneBB.Extend(mgl64.Vec3{deltaX, deltaY, deltaZ}))

		for _, blockBBox := range boxes {
			deltaY = stepBB.YOffset(blockBBox, deltaY)
		}
		stepBB = stepBB.Translate(mgl64.Vec3{0, deltaY})

		for _, blockBBox := range boxes {
			deltaX = stepBB.XOffset(blockBBox, deltaX)
		}
		stepBB = stepBB.Translate(mgl64.Vec3{deltaX})
		for _, blockBBox := range boxes {
			deltaZ = stepBB.ZOffset(blockBBox, deltaZ)
		}
		stepBB = stepBB.Translate(mgl64.Vec3{0, 0, deltaZ})

		reverseDeltaY := -deltaY
		for _, blockBBox := range boxes {
			reverseDeltaY = stepBB.YOffset(blockBBox, reverseDeltaY)
		}
		deltaY += reverseDeltaY

		if (math.Pow(cx, 2)+math.Pow(cz, 2)) >= (math.Pow(deltaX, 2)+math.Pow(deltaZ, 2)) || mgl64.FloatEqual(deltaY, 0) {
			deltaX, deltaY, deltaZ = cx, cy, cz
		} else {
			p.mInfo.StepLenience += deltaY
			moveBB = stepBB
		}
	}

	p.mInfo.OnGround = false
	p.mInfo.VerticallyCollided = !mgl64.FloatEqual(vel[1], deltaY)
	if p.mInfo.VerticallyCollided {
		p.mInfo.OnGround = vel[1] < 0
		vel[1] = 0
	}

	p.mInfo.XCollision = !mgl64.FloatEqual(vel[0], deltaX)
	if p.mInfo.XCollision {
		vel[0] = 0
	}

	p.mInfo.ZCollision = !mgl64.FloatEqual(vel[2], deltaZ)
	if p.mInfo.ZCollision {
		vel[2] = 0
	}

	p.mInfo.HorizontallyCollided = p.mInfo.XCollision || p.mInfo.ZCollision
	p.mInfo.ServerMovement = vel

	min, max := moveBB.Min(), moveBB.Max()
	p.mInfo.ServerPosition = mgl64.Vec3{
		(min[0] + max[0]) / 2,
		min[1] - p.mInfo.StepLenience,
		(min[2] + max[2]) / 2,
	}
	if p.mInfo.StepLenience > 1e-4 {
		p.mInfo.ServerPosition = p.Position() // TODO! __Proper__ step predictions
	}

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
		p.mInfo.ServerPosition = p.Position()
		p.mInfo.ServerMovement = p.mInfo.ClientPredictedMovement
	}
}

// simulateGravity simulates the gravity of the player
func (p *Player) simulateGravity() {
	p.mInfo.ServerMovement[1] -= p.mInfo.Gravity
	p.mInfo.ServerMovement[1] *= 0.98
}

// simulateHorizontalFriction simulates the horizontal friction of the player
func (p *Player) simulateHorizontalFriction(friction float64) {
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

type MovementInfo struct {
	CanExempt bool

	InUnsupportedRewindScenario bool

	MoveForward, MoveStrafe float64
	JumpVelocity            float64
	Gravity                 float64
	Speed                   float64
	StepLenience            float64

	MotionTicks uint64

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

	ClientPredictedMovement, ClientMovement mgl64.Vec3
	ServerSentMovement                      mgl64.Vec3
	ServerMovement, OldServerMovement       mgl64.Vec3
	ServerPosition                          mgl64.Vec3
}

func (m *MovementInfo) UpdateServerSentVelocity(velo mgl64.Vec3) {
	m.ServerSentMovement = velo
	m.MotionTicks = 0
}

func (m *MovementInfo) UpdateTickStatus() {
	m.MotionTicks++
}
