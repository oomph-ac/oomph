package simulation

import (
	"strings"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type MovementSimulator struct {
}

func (s MovementSimulator) Simulate(p *player.Player) {
	p.Dbg.Notify(player.DebugModeMovementSim, true, "START movement simulation for tick %d", p.ClientFrame)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement simulation for tick %d", p.ClientFrame)

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	mDat.Scenarios = nil
	mDat.Scenarios = []handler.MovementScenario{}

	if !s.Reliable(p) {
		mDat.Velocity = mDat.ClientVel
		mDat.PrevVelocity = mDat.PrevClientVel
		mDat.Position = mDat.ClientPosition
		mDat.Mov = mDat.ClientMov
		mDat.OnGround = true
		mDat.Scenarios = append(mDat.Scenarios, mDat.MovementScenario)
		return
	}

	// The player at this point should have already sent a tick sync to the server.
	if !p.Ready {
		p.Disconnect(game.ErrorNotReady)
		return
	}

	for i := handler.SimulationNormal; i <= handler.SimulationAccountingGhostBlock; i++ {
		s.doActualSimulation(p, i)
	}

	var sc handler.MovementScenario
	minDev := float32(math32.MaxFloat32 - 1)
	for _, s := range mDat.Scenarios {
		dev := s.Position.Sub(mDat.ClientPosition).LenSqr()
		if dev < minDev {
			minDev = dev
			sc = s
		}
	}

	mDat.MovementScenario = sc

	// If the position between the server and client deviates more than the correction threshold
	// in a tick, correct their movement, and don't accept any client movement until the position has been syncrohnised..
	// We also correct the movement if the player is in a ghost block scenario, as they are not synced with
	// the server's current state, but are not necessarily cheating.
	if mDat.Position.Sub(mDat.ClientPosition).Len() >= mDat.CorrectionThreshold ||
		mDat.MovementScenario.ID == handler.SimulationAccountingGhostBlock ||
		(!p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).InLoadedChunk && mDat.TicksSinceTeleport >= 100) {
		mDat.CorrectMovement(p)
	}
}

func (s MovementSimulator) doActualSimulation(p *player.Player, run int) {
	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN run %d", run)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END run %d", run)

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	w := p.World

	if run == handler.SimulationAccountingGhostBlock && !w.HasGhostBlocks() {
		return
	}

	oldS := mDat.MovementScenario
	defer func() {
		mDat.Scenarios = append(mDat.Scenarios, handler.MovementScenario{
			ID:                   run,
			Position:             mDat.Position,
			Velocity:             mDat.Velocity,
			OnGround:             mDat.OnGround,
			OffGroundTicks:       mDat.OffGroundTicks,
			CollisionX:           mDat.CollisionX,
			CollisionZ:           mDat.CollisionZ,
			VerticallyCollided:   mDat.VerticallyCollided,
			HorizontallyCollided: mDat.HorizontallyCollided,
			KnownInsideBlock:     mDat.KnownInsideBlock,
		})
		mDat.MovementScenario = oldS
	}()

	// Do not allow the player to move if not in a loaded chunk.
	if !p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).InLoadedChunk {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player not in loaded chunk")
		return
	}

	if run == handler.SimulationAccountingGhostBlock {
		w.SearchWithGhost(true)
		defer w.SearchWithGhost(false)
	}

	if mDat.Immobile {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player is immobile")
		mDat.Velocity = mgl32.Vec3{}
		return
	}

	// If a teleport was able to be handled, do not continue with the simulation.
	if s.teleport(mDat) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", mDat.Position)
		return
	}

	// Push the player out of any blocks they may be in.
	old := mDat.Position
	s.pushOutOfBlocks(mDat, w)
	p.Dbg.Notify(player.DebugModeMovementSim, !old.ApproxEqual(mDat.Position), "pushOutOfBlocks (oldPos=%v, newPos=%v)", old, mDat.Position)

	// Reset the velocity to zero if it's significantly small.
	if mDat.Velocity.LenSqr() < 1e-12 {
		mDat.Velocity = mgl32.Vec3{}
	}

	// Apply knockback if applicable.
	p.Dbg.Notify(player.DebugModeMovementSim, s.knockback(mDat), "knockback applied: %v", mDat.Velocity)
	p.Dbg.Notify(player.DebugModeMovementSim, s.jump(mDat), "jump force applied (sprint=%v): %v", mDat.Sprinting, mDat.Velocity)

	mDat.StepClipOffset *= game.StepClipMultiplier
	if mDat.StepClipOffset < 1e-7 {
		mDat.StepClipOffset = 0
	}

	blockUnder := w.GetBlock(cube.PosFromVec3(mDat.Position.Sub(mgl32.Vec3{0, 0.5})))
	blockFriction := game.DefaultAirFriction
	v3 := mDat.AirSpeed
	if mDat.OnGround {
		blockFriction *= utils.BlockFriction(blockUnder)
		v3 = mDat.MovementSpeed * (0.162771336 / math32.Pow(blockFriction, 3))
	}

	mDat.Friction = blockFriction
	p.Dbg.Notify(player.DebugModeMovementSim, true, "blockUnder=%s, blockFriction=%v, v3=%v", utils.BlockName(blockUnder), blockFriction, v3)
	s.moveRelative(mDat, v3)
	p.Dbg.Notify(player.DebugModeMovementSim, true, "moveRelative force applied (vel=%v)", mDat.Velocity)

	nearClimable := utils.BlockClimbable(w.GetBlock(cube.PosFromVec3(mDat.Position)))
	if nearClimable {
		mDat.Velocity[0] = game.ClampFloat(mDat.Velocity[0], -0.2, 0.2)
		mDat.Velocity[2] = game.ClampFloat(mDat.Velocity[2], -0.2, 0.2)
		if mDat.Velocity[1] < -0.2 {
			mDat.Velocity[1] = -0.2
		}
		if mDat.Sneaking && mDat.Velocity[1] < 0 {
			mDat.Velocity[1] = 0
		}

		p.Dbg.Notify(player.DebugModeMovementSim, true, "climb velocity applied (vel=%v)", mDat.Velocity)
		mDat.Climb = true
	}

	blocksInside, inside := s.blocksInside(mDat, w)
	inCobweb := false
	if inside {
		for _, b := range blocksInside {
			if utils.BlockName(b) == "minecraft:web" {
				inCobweb = true
				break
			}
		}
	}

	if inCobweb {
		mDat.Velocity[0] *= 0.25
		mDat.Velocity[1] *= 0.05
		mDat.Velocity[2] *= 0.25
		p.Dbg.Notify(player.DebugModeMovementSim, true, "cobweb force applied (vel=%v)", mDat.Velocity)
	}

	// Avoid edges if the player is sneaking on the edge of a block.
	old = mDat.Position
	s.avoidEdge(mDat, w)
	p.Dbg.Notify(player.DebugModeMovementSim, !old.ApproxEqual(mDat.Position), "avoidEdge (oldPos=%v, newPos=%v)", old, mDat.Position)

	oldVel := mDat.Velocity
	s.collide(mDat, w)
	p.Dbg.Notify(player.DebugModeMovementSim, !oldVel.ApproxEqual(mDat.Velocity), "collide (oldVel=%v, newVel=%v)", oldVel, mDat.Velocity)

	isClimb := nearClimable && (mDat.HorizontallyCollided || mDat.JumpKeyPressed)
	if isClimb {
		mDat.Velocity[1] = game.ClimbSpeed
		p.Dbg.Notify(player.DebugModeMovementSim, true, "upward climb applied")
	}

	mDat.Position = mDat.Position.Add(mDat.Velocity)
	mDat.Mov = mDat.Velocity
	p.Dbg.Notify(player.DebugModeMovementSim, true, "result (pos=%v vel=%v)", mDat.Position, mDat.Velocity)

	blockUnder = w.GetBlock(cube.PosFromVec3(mDat.Position.Sub(mgl32.Vec3{0, 0.2})))
	if _, isAir := blockUnder.(block.Air); isAir {
		b := w.GetBlock(cube.PosFromVec3(mDat.Position).Side(cube.FaceDown))
		n := utils.BlockName(b)
		if utils.IsFence(b) || utils.IsWall(n) || strings.Contains(n, "fence") {
			blockUnder = b
		}
	}

	s.checkCollisions(mDat, oldVel, isClimb, blockUnder)
	s.walkOnBlock(mDat, blockUnder)

	if inCobweb {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move cobweb force applied (0 vel)")
		mDat.Velocity = mgl32.Vec3{}
	}

	// Apply gravity.
	mDat.Velocity[1] -= mDat.Gravity
	mDat.Velocity[1] *= game.GravityMultiplier

	// Apply friction.
	mDat.Velocity[0] *= blockFriction
	mDat.Velocity[2] *= blockFriction

	p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move final velocity: %v", mDat.Velocity)
}

func (MovementSimulator) Reliable(p *player.Player) bool {
	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	for _, b := range utils.GetNearbyBlocks(mDat.BoundingBox(), false, true, p.World) {
		if _, isLiquid := b.Block.(df_world.Liquid); isLiquid {
			return false
		}

		if utils.BlockName(b.Block) == "minecraft:bamboo" {
			return false
		}
	}

	return (p.GameMode == packet.GameTypeSurvival || p.GameMode == packet.GameTypeAdventure) &&
		!mDat.Flying &&
		!mDat.NoClip &&
		p.Alive &&
		mDat.Position.Y() >= -64
}

func (s MovementSimulator) teleport(mDat *handler.MovementHandler) bool {
	if !mDat.SmoothTeleport && mDat.TicksSinceTeleport == 0 {
		mDat.Position = mDat.TeleportPos
		mDat.Velocity = mgl32.Vec3{}
		if mDat.TeleportOnGround {
			mDat.Velocity[1] = -0.002
		}
		mDat.OnGround = mDat.TeleportOnGround
		mDat.TicksUntilNextJump = 0
		s.jump(mDat)
		return true
	} else if mDat.SmoothTeleport && mDat.TicksSinceTeleport <= 3 {
		mDat.Velocity[1] = -0.078
		delta := mDat.TeleportPos.Sub(mDat.Position)
		if mDat.TicksSinceTeleport < 2 {
			mDat.Position = mDat.Position.Add(delta.Mul(1.0 / float32(3-mDat.TicksSinceTeleport)))
		} else {
			mDat.Position = mDat.TeleportPos
			mDat.OnGround = mDat.TeleportOnGround
		}

		return true
	}

	return false
}

func (MovementSimulator) jump(mDat *handler.MovementHandler) bool {
	if !mDat.Jumping || !mDat.OnGround || mDat.TicksUntilNextJump > 0 {
		mDat.Jumped = false
		return false
	}

	mDat.Jumped = true
	mDat.Velocity[1] = mDat.JumpHeight
	mDat.TicksUntilNextJump = game.JumpDelayTicks
	if !mDat.Sprinting {
		return true
	}

	force := mDat.Rotation.Z() * 0.017453292
	mDat.Velocity[0] -= game.MCSin(force) * 0.2
	mDat.Velocity[2] += game.MCCos(force) * 0.2
	return true
}

func (MovementSimulator) pushOutOfBlocks(mDat *handler.MovementHandler, w *world.World) {
	if mDat.StepClipOffset > 0 {
		return
	}

	blockIn := w.GetBlock(cube.PosFromVec3(mDat.Position))
	if utils.CanPassBlock(blockIn) {
		mDat.KnownInsideBlock = false
		return
	}

	inside := false
	playerBB := mDat.BoundingBox()
	newPos := mDat.Position

	airBlocks := map[cube.Face]bool{}
	for _, face := range cube.Faces() {
		facedBlockPos := cube.PosFromVec3(mDat.Position).Side(face)
		_, isAir := w.GetBlock(facedBlockPos).(block.Air)
		airBlocks[face] = isAir
	}

	for _, result := range utils.GetNearbyBlocks(mDat.BoundingBox(), false, true, w) {
		if utils.CanPassBlock(result.Block) {
			continue
		}

		for _, box := range utils.BlockBoxes(result.Block, result.Position, w) {
			box = box.Translate(result.Position.Vec3())
			if !playerBB.IntersectsWith(box) {
				continue
			}

			if airBlocks[cube.FaceUp] && playerBB.Min().Y() < box.Max().Y() && mDat.Mov.Y() <= 0 {
				newPos[1] = box.Max().Y() + 1e-3
				inside = true
				continue
			} else if airBlocks[cube.FaceDown] && !airBlocks[cube.FaceUp] && playerBB.Max().Y() > box.Min().Y() {
				if box.Height() <= 0.5 {
					newPos[1] = box.Max().Y() + 1e-3
					inside = true
				} else {
					newPos[1] = box.Min().Y() - 1e-3
					inside = true
				}

				continue
			}

			if airBlocks[cube.FaceWest] && playerBB.Max().X()-box.Min().X() > 0 && box.Max().X()-playerBB.Min().X() <= 0.5 {
				newPos[0] = box.Max().X() + 0.5
				inside = true
			} else if airBlocks[cube.FaceEast] && box.Max().X()-playerBB.Min().X() > 0 && playerBB.Max().X()-box.Min().X() >= -0.5 {
				newPos[0] = box.Min().X() - 0.5
				inside = true
			}

			if airBlocks[cube.FaceNorth] && playerBB.Max().Z()-box.Min().Z() > 0 && box.Max().Z()-playerBB.Min().Z() <= 0.5 {
				newPos[2] = box.Max().Z() + 0.5
				inside = true
			} else if airBlocks[cube.FaceSouth] && box.Max().Z()-playerBB.Min().Z() > 0 && playerBB.Max().Z()-box.Min().Z() >= -0.5 {
				newPos[2] = box.Min().Z() - 0.5
				inside = true
			}
		}
	}

	if !mDat.KnownInsideBlock && inside {
		mDat.Position = newPos
	}

	mDat.KnownInsideBlock = inside
}

func (MovementSimulator) knockback(mDat *handler.MovementHandler) bool {
	if mDat.TicksSinceKnockback != 0 {
		return false
	}
	mDat.Velocity = mDat.Knockback
	return true
}

func (MovementSimulator) moveRelative(mDat *handler.MovementHandler, fSpeed float32) {
	v := math32.Pow(mDat.ForwardImpulse, 2) + math32.Pow(mDat.LeftImpulse, 2)
	if v < 1e-4 {
		return
	}

	v = math32.Sqrt(v)
	if v < 1 {
		v = 1
	}
	v = fSpeed / v

	mf, ms := mDat.ForwardImpulse*v, mDat.LeftImpulse*v
	force := mDat.Rotation.Z() * (math32.Pi / 180)
	v2, v3 := game.MCSin(force), game.MCCos(force)

	mDat.Velocity[0] += ms*v3 - mf*v2
	mDat.Velocity[2] += ms*v2 + mf*v3
}

func (MovementSimulator) blocksInside(mDat *handler.MovementHandler, w *world.World) ([]df_world.Block, bool) {
	bb := mDat.BoundingBox()
	blocks := []df_world.Block{}

	for _, result := range utils.GetNearbyBlocks(bb, false, true, w) {
		pos := result.Position
		block := result.Block
		boxes := utils.BlockBoxes(block, pos, w)

		for _, box := range boxes {
			if bb.IntersectsWith(box.Translate(pos.Vec3())) {
				blocks = append(blocks, block)
			}
		}
	}

	return blocks, len(blocks) > 0
}

func (MovementSimulator) avoidEdge(mDat *handler.MovementHandler, w *world.World) {
	if !mDat.Sneaking || !mDat.OnGround || mDat.Velocity.Y() > 0 {
		return
	}

	currentVel := mDat.Velocity
	bb := mDat.BoundingBox().GrowVec3(mgl32.Vec3{-0.025, 0, -0.025})
	xMov, zMov, offset := currentVel.X(), currentVel.Z(), float32(0.05)

	for xMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -1, 0}), w)) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}
	}

	for zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -1, zMov}), w)) == 0 {
		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	for xMov != 0.0 && zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -1, zMov}), w)) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}

		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	mDat.Velocity[0] = xMov
	mDat.Velocity[2] = zMov
}

func (s MovementSimulator) collide(mDat *handler.MovementHandler, w *world.World) {
	currVel := mDat.Velocity
	newVel := currVel
	bbList := utils.GetNearbyBBoxes(mDat.BoundingBox().Extend(currVel), w)

	if currVel.LenSqr() == 0.0 {
		return
	}

	newVel = s.collideWithBoxes(mDat.BoundingBox(), currVel, bbList)
	xCollision := currVel.X() != newVel.X()
	yCollision := currVel.Y() != newVel.Y()
	zCollision := currVel.Z() != newVel.Z()
	onGround := mDat.OnGround || (yCollision && currVel.Y() < 0.0)

	if onGround && (xCollision || zCollision) {
		stepVel := mgl32.Vec3{currVel.X(), game.StepHeight, currVel.Z()}
		list := utils.GetNearbyBBoxes(mDat.BoundingBox().Extend(stepVel), w)

		var bb cube.BBox
		bb, stepVel[1] = utils.DoBoxCollision(utils.CollisionY, mDat.BoundingBox(), list, stepVel[1])
		bb, stepVel[0] = utils.DoBoxCollision(utils.CollisionX, bb, list, stepVel[0])
		bb, stepVel[2] = utils.DoBoxCollision(utils.CollisionZ, bb, list, stepVel[2])
		_, rDy := utils.DoBoxCollision(utils.CollisionY, bb, bbList, -(stepVel.Y()))
		stepVel[1] += rDy

		if game.Vec3HzDistSqr(newVel) < game.Vec3HzDistSqr(stepVel) {
			mDat.StepClipOffset += stepVel.Y()
			newVel = stepVel
		}
	}

	mDat.Velocity = newVel
}

func (MovementSimulator) collideWithBoxes(bb cube.BBox, vel mgl32.Vec3, list []cube.BBox) mgl32.Vec3 {
	if len(list) == 0 {
		return vel
	}

	xVel, yVel, zVel := vel.X(), vel.Y(), vel.Z()
	if yVel != 0 {
		bb, yVel = utils.DoBoxCollision(utils.CollisionY, bb, list, yVel)
	}

	flag := math32.Abs(xVel) < math32.Abs(zVel)
	if flag && zVel != 0 {
		bb, zVel = utils.DoBoxCollision(utils.CollisionZ, bb, list, zVel)
	}

	if xVel != 0 {
		bb, xVel = utils.DoBoxCollision(utils.CollisionX, bb, list, xVel)
	}

	if !flag && zVel != 0 {
		_, zVel = utils.DoBoxCollision(utils.CollisionZ, bb, list, zVel)
	}

	return mgl32.Vec3{xVel, yVel, zVel}
}

func (s MovementSimulator) checkCollisions(mDat *handler.MovementHandler, old mgl32.Vec3, climb bool, blockUnder df_world.Block) {
	mDat.CollisionX = !mgl32.FloatEqualThreshold(mDat.Velocity.X(), old.X(), 1e-5)
	mDat.CollisionZ = !mgl32.FloatEqualThreshold(mDat.Velocity.Z(), old.Z(), 1e-5)
	mDat.HorizontallyCollided = mDat.CollisionX || mDat.CollisionZ

	mDat.VerticallyCollided = mDat.Velocity.Y() != old.Y()
	mDat.OnGround = mDat.VerticallyCollided && old.Y() < 0.0

	if climb {
		return
	}

	if mDat.VerticallyCollided {
		s.landOnBlock(mDat, old, blockUnder)
	}

	if mDat.CollisionX {
		mDat.Velocity[0] = 0
	}

	if mDat.CollisionZ {
		mDat.Velocity[2] = 0
	}
}

func (MovementSimulator) landOnBlock(mDat *handler.MovementHandler, old mgl32.Vec3, blockUnder df_world.Block) {
	if !mDat.OnGround || old.Y() >= 0 || mDat.SneakKeyPressed {
		mDat.Velocity[1] = 0
		return
	}

	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		mDat.Velocity[1] = game.SlimeBounceMultiplier * old.Y()
	case "minecraft:bed":
		mDat.Velocity[1] = game.BedBounceMultiplier * old.Y()
	default:
		mDat.Velocity[1] = 0
	}
}

func (MovementSimulator) walkOnBlock(mDat *handler.MovementHandler, blockUnder df_world.Block) {
	if !mDat.OnGround || mDat.Sneaking {
		return
	}

	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		yMov := math32.Abs(mDat.Velocity.Y())
		if yMov < 0.1 && !mDat.SneakKeyPressed {
			d1 := 0.4 + yMov*0.2
			mDat.Velocity[0] *= d1
			mDat.Velocity[2] *= d1
		}
	case "minecraft:soul_sand":
		mDat.Velocity[0] *= 0.3
		mDat.Velocity[2] *= 0.3
	}
}
