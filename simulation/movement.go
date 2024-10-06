package simulation

import (
	"strings"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
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

	if !s.Reliable(p) {
		mDat.Velocity = mDat.ClientVel
		mDat.PrevVelocity = mDat.PrevClientVel
		mDat.Position = mDat.ClientPosition
		mDat.Mov = mDat.ClientMov
		mDat.OnGround = true
		return
	}

	// The player at this point should have already sent a tick sync to the server.
	if !p.Ready {
		p.Disconnect(game.ErrorNotReady)
		return
	}

	s.doActualSimulation(p)

	// If the position between the server and client deviates more than the correction threshold
	// in a tick, correct their movement, and don't accept any client movement until the position has been syncrohnised..
	if mDat.Position.Sub(mDat.ClientPosition).Len() >= mDat.CorrectionThreshold ||
		(!p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).InLoadedChunk && mDat.TicksSinceTeleport >= 100) {
		mDat.CorrectMovement(p)
		return
	}
}

func (s MovementSimulator) doActualSimulation(p *player.Player) {
	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN movement sim")
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement sim")

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	w := p.World

	// Do not allow the player to move if not in a loaded chunk.
	if !p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).InLoadedChunk {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player not in loaded chunk")
		return
	}

	// If a teleport was able to be handled, do not continue with the simulation.
	if s.teleport(mDat) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", mDat.Position)
		return
	}

	if mDat.Immobile {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player is immobile")
		mDat.Velocity = mgl32.Vec3{}
		return
	}

	// Push the player out of any blocks they may be in.
	/* old := mDat.Position
	s.pushOutOfBlocks(mDat, w)
	p.Dbg.Notify(player.DebugModeMovementSim, !old.ApproxEqual(mDat.Position), "pushOutOfBlocks (oldPos=%v, newPos=%v)", old, mDat.Position) */

	// Reset the velocity to zero if it's significantly small.
	if mDat.Velocity.LenSqr() < 1e-12 {
		mDat.Velocity = mgl32.Vec3{}
	}

	// Apply knockback if applicable.
	p.Dbg.Notify(player.DebugModeMovementSim, s.knockback(mDat), "knockback applied: %v", mDat.Velocity)
	p.Dbg.Notify(player.DebugModeMovementSim, s.jump(mDat), "jump force applied (sprint=%v): %v", mDat.Sprinting, mDat.Velocity)

	blockUnder := w.Block(df_cube.Pos(cube.PosFromVec3(mDat.Position.Sub(mgl32.Vec3{0, 0.5}))))
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

	nearClimable := utils.BlockClimbable(w.Block(df_cube.Pos(cube.PosFromVec3(mDat.Position))))
	if nearClimable && !mDat.JumpKeyPressed {
		mDat.Velocity[0] = game.ClampFloat(mDat.Velocity[0], -0.3, 0.3)
		mDat.Velocity[2] = game.ClampFloat(mDat.Velocity[2], -0.3, 0.3)
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
	old := mDat.Position
	s.avoidEdge(mDat, w)
	p.Dbg.Notify(player.DebugModeMovementSim, !old.ApproxEqual(mDat.Position), "avoidEdge (oldPos=%v, newPos=%v)", old, mDat.Position)

	oldVel := mDat.Velocity
	s.tryCollisions(mDat, w, p.Dbg)
	mDat.Mov = mDat.Velocity

	blockUnder = w.Block(df_cube.Pos(cube.PosFromVec3(mDat.Position.Sub(mgl32.Vec3{0, 0.2}))))
	if _, isAir := blockUnder.(block.Air); isAir {
		b := w.Block(df_cube.Pos(cube.PosFromVec3(mDat.Position).Side(cube.FaceDown)))
		n := utils.BlockName(b)
		if utils.IsFence(b) || utils.IsWall(n) || strings.Contains(n, "fence") {
			blockUnder = b
		}
	}

	isClimb := nearClimable && (mDat.JumpKeyPressed || mDat.HorizontallyCollided)
	s.trySetPostCollisionMotion(mDat, oldVel, isClimb, blockUnder)
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

	if isClimb {
		mDat.Velocity[1] = game.ClimbSpeed
		p.Dbg.Notify(player.DebugModeMovementSim, true, "upward climb applied")
	}

	mDat.SlideOffset = mDat.SlideOffset.Mul(0.4)
	mDat.SlideOffset[0] = game.Round32(mDat.SlideOffset[0], 5)
	mDat.SlideOffset[1] = game.Round32(mDat.SlideOffset[1], 5)

	p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move final velocity: %v", mDat.Velocity)
}

func (MovementSimulator) Reliable(p *player.Player) bool {
	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)

	// Always simulate teleports sent by the server to prevent de-sync.
	if mDat.TicksSinceTeleport < mDat.TeleportTicks() {
		return true
	}

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
	bb := mDat.BoundingBox().
		GrowVec3(mgl32.Vec3{-0.025, 0, -0.025})
	xMov, zMov, offset := currentVel.X(), currentVel.Z(), float32(0.05)

	for xMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, 0}), w)) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}
	}

	for zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -game.StepHeight * 1.01, zMov}), w)) == 0 {
		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	for xMov != 0.0 && zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, zMov}), w)) == 0 {
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

func (s MovementSimulator) tryCollisions(mDat *handler.MovementHandler, w *world.World, dbg *player.Debugger) {
	currVel := mDat.Velocity
	collisionBB := mDat.BoundingBox()
	bbList := utils.GetNearbyBBoxes(mDat.BoundingBox().Extend(currVel), w)
	oneWayBlocks := utils.OneWayCollisionBlocks(utils.GetNearbyBlocks(mDat.BoundingBox().Extend(currVel), false, false, w))

	// TODO: determine more blocks that are considered to be one-way physics blocks, and
	// figure out how to calculate ActorCollision::isStuckItem()
	oneWayCollisions := len(oneWayBlocks) != 0 || mDat.StuckInCollider
	penetration := mgl32.Vec3{}
	dbg.Notify(player.DebugModeMovementSim, oneWayCollisions, "one-way collisions are used for this simulation")

	v1 := mgl32.Vec3{0, currVel.Y()}
	v2 := mgl32.Vec3{currVel.X()}
	v3 := mgl32.Vec3{0, 0, currVel.Z()}

	for _, blockBox := range bbList {
		v1 = game.BBClipCollide(blockBox, collisionBB, v1, oneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(mgl32.Vec3{0, v1.Y()})
	dbg.Notify(player.DebugModeMovementSim, true, "Y-collision non-step=%f /w penetration=%f", v1.Y(), penetration.Y())

	for _, blockBox := range bbList {
		v2 = game.BBClipCollide(blockBox, collisionBB, v2, oneWayCollisions, &penetration)
	}
	collisionBB = collisionBB.Translate(mgl32.Vec3{v2.X()})
	dbg.Notify(player.DebugModeMovementSim, true, "X-collision non-step=%f /w penetration=%f", v2.X(), penetration.X())

	for _, blockBox := range bbList {
		v3 = game.BBClipCollide(blockBox, collisionBB, v3, oneWayCollisions, &penetration)
	}
	dbg.Notify(player.DebugModeMovementSim, true, "Z-collision non-step=%f /w penetration=%f", v3.Z(), penetration.Z())

	collisionVel := mgl32.Vec3{v2.X(), v1.Y(), v3.Z()}
	hasPenetration := penetration.LenSqr() >= 9.999999999999999e-12
	mDat.StuckInCollider = mDat.PenetratingLastFrame && hasPenetration
	mDat.PenetratingLastFrame = hasPenetration

	xCollision := currVel.X() != collisionVel.X()
	yCollision := currVel.Y() != collisionVel.Y()
	zCollision := currVel.Z() != collisionVel.Z()
	onGround := mDat.OnGround || (yCollision && currVel.Y() < 0.0)

	if onGround && (xCollision || zCollision) {
		v172 := mgl32.Vec3{0, game.StepHeight}
		v174 := mgl32.Vec3{currVel.X()}
		v173 := mgl32.Vec3{0, 0, currVel.Z()}

		stepBB := mDat.BoundingBox()
		for _, blockBox := range bbList {
			v172 = game.BBClipCollide(blockBox, stepBB, v172, oneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(v172)

		for _, blockBox := range bbList {
			v174 = game.BBClipCollide(blockBox, stepBB, v174, oneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(v174)

		for _, blockBox := range bbList {
			v173 = game.BBClipCollide(blockBox, stepBB, v173, oneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(v173)

		v175 := v172.Mul(-1)
		for _, blockBox := range bbList {
			v175 = game.BBClipCollide(blockBox, stepBB, v175, oneWayCollisions, nil)
		}
		v172 = v172.Add(v175)

		finalVel := v172.Add(v173).Add(v174)
		newBBList := utils.GetNearbyBBoxes(stepBB, w)
		dbg.Notify(player.DebugModeMovementSim, true, "newBBList count: %d", len(newBBList))

		if len(newBBList) == 0 && game.Vec3HzDistSqr(collisionVel) < game.Vec3HzDistSqr(finalVel) {
			collisionVel = finalVel
			dbg.Notify(player.DebugModeMovementSim, true, "step successful: %v", collisionVel)
			//mDat.SlideOffset[1] += v172.Y()
		}
	}

	mDat.CollisionX = math32.Abs(currVel.X()-collisionVel.X()) >= 1e-5
	mDat.CollisionZ = math32.Abs(currVel.Z()-collisionVel.Z()) >= 1e-5
	mDat.HorizontallyCollided = mDat.CollisionX || mDat.CollisionZ

	mDat.VerticallyCollided = math32.Abs(currVel.Y()-collisionVel.Y()) >= 1e-5
	mDat.OnGround = (mDat.VerticallyCollided && currVel.Y() < 0) || (mDat.OnGround && !mDat.VerticallyCollided && currVel.Y() == 0.0)

	mDat.Velocity = collisionVel
	mDat.Position = mDat.Position.Add(collisionVel)
	//mDat.Position[1] -= mDat.SlideOffset.Y()
	dbg.Notify(player.DebugModeMovementSim, true, "finalVel=%v finalPos=%v", mDat.Velocity, mDat.Position)
}

func (s MovementSimulator) trySetPostCollisionMotion(mDat *handler.MovementHandler, old mgl32.Vec3, climb bool, blockUnder df_world.Block) {
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
