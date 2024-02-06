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
	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	w := p.World

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

	if mDat.Immobile {
		mDat.Velocity = mgl32.Vec3{}
		return
	}

	// If a teleport was able to be handled, do not continue with the simulation.
	if s.teleport(p, mDat) {
		return
	}

	// Push the player out of any blocks they may be in.
	s.pushOutOfBlocks(mDat, w)

	// Reset the velocity to zero if it's significantly small.
	if mDat.Velocity.LenSqr() < 1e-12 {
		mDat.Velocity = mgl32.Vec3{}
	}

	// Apply knockback if applicable.
	s.knockback(mDat)
	s.jump(mDat)

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
	s.moveRelative(mDat, v3)

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
	}

	b, inside := s.blockInside(mDat, w)
	inCobweb := inside && utils.BlockName(b) == "minecraft:web"

	if inCobweb {
		mDat.Velocity[0] *= 0.25
		mDat.Velocity[1] *= 0.05
		mDat.Velocity[2] *= 0.25
	}

	// Avoid edges if the player is sneaking on the edge of a block.
	s.avoidEdge(mDat, w)
	oldVel := mDat.Velocity

	s.collide(mDat, w)
	isClimb := nearClimable && (mDat.HorizontallyCollided || mDat.JumpKeyPressed)
	if isClimb {
		mDat.Velocity[1] = game.ClimbSpeed
	}

	mDat.Position = mDat.Position.Add(mDat.Velocity)
	mDat.Mov = mDat.Velocity

	blockUnder = w.GetBlock(cube.PosFromVec3(mDat.Position.Sub(mgl32.Vec3{0, 0.2})))
	if _, isAir := blockUnder.(block.Air); isAir {
		b := w.GetBlock(cube.PosFromVec3(mDat.Position).Side(cube.FaceDown))
		n := utils.BlockName(b)
		if utils.IsFence(n) || utils.IsWall(n) || strings.Contains(n, "fence_gate") {
			blockUnder = b
		}
	}

	s.checkCollisions(mDat, oldVel, isClimb, blockUnder)
	s.walkOnBlock(mDat, blockUnder)

	if inCobweb {
		mDat.Velocity = mgl32.Vec3{}
	}

	// Apply gravity.
	mDat.Velocity[1] -= mDat.Gravity
	mDat.Velocity[1] *= game.GravityMultiplier

	// Apply friction.
	mDat.Velocity[0] *= blockFriction
	mDat.Velocity[2] *= blockFriction
}

func (MovementSimulator) Reliable(p *player.Player) bool {
	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	cDat := p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler)

	for _, b := range utils.GetNearbyBlocks(mDat.BoundingBox().Grow(1), false, false, p.World) {
		if _, isLiquid := b.Block.(df_world.Liquid); isLiquid {
			return false
		}

		if utils.BlockName(b.Block) == "minecraft:bamboo" {
			return false
		}
	}

	return (p.GameMode == packet.GameTypeSurvival || p.GameMode == packet.GameTypeAdventure) && !mDat.Flying &&
		!mDat.NoClip && p.Alive && cDat.InLoadedChunk
}

func (s MovementSimulator) teleport(p *player.Player, mDat *handler.MovementHandler) bool {
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

func (MovementSimulator) jump(mDat *handler.MovementHandler) {
	if !mDat.Jumping || !mDat.OnGround || mDat.TicksUntilNextJump > 0 {
		return
	}

	mDat.Velocity[1] = mDat.JumpHeight
	mDat.TicksUntilNextJump = game.JumpDelayTicks
	if !mDat.Sprinting {
		return
	}

	force := mDat.Rotation.Z() * 0.017453292
	mDat.Velocity[0] -= game.MCSin(force) * 0.2
	mDat.Velocity[2] += game.MCCos(force) * 0.2
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
	bb := mDat.BoundingBox()
	newPos := mDat.Position

	airBlocks := map[cube.Face]bool{}
	for _, face := range cube.Faces() {
		facedBlockPos := cube.PosFromVec3(mDat.Position).Side(face)
		_, isAir := w.GetBlock(facedBlockPos).(block.Air)
		airBlocks[face] = isAir
	}

	for _, result := range utils.GetNearbyBlocks(mDat.BoundingBox(), false, true, w) {
		if !utils.CanPassBlock(result.Block) {
			continue
		}

		for _, box := range utils.BlockBoxes(result.Block, result.Position, w) {
			box = box.Translate(result.Position.Vec3())
			if !bb.IntersectsWith(box) {
				continue
			}

			minDelta, maxDelta := box.Max().Sub(bb.Min()), box.Min().Sub(bb.Max())
			if !airBlocks[cube.FaceUp] && box.Max().Y()-bb.Min().Y() > 0 && minDelta.Y() <= 0.5 {
				newPos[1] = box.Max().Y() + 1e-3
				inside = true
			}

			if !airBlocks[cube.FaceWest] && bb.Max().X()-box.Min().X() > 0 && minDelta.X() <= 0.5 {
				newPos[0] = box.Max().X() + 0.5
				inside = true
			} else if !airBlocks[cube.FaceEast] && box.Max().X()-bb.Min().X() > 0 && maxDelta.X() >= -0.5 {
				newPos[0] = box.Min().X() - 0.5
				inside = true
			}

			if !airBlocks[cube.FaceNorth] && bb.Max().Z()-box.Min().Z() > 0 && minDelta.Z() <= 0.5 {
				newPos[2] = box.Max().Z() + 0.5
				inside = true
			} else if !airBlocks[cube.FaceSouth] && box.Max().Z()-bb.Min().Z() > 0 && maxDelta.Z() >= -0.5 {
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

func (MovementSimulator) knockback(mDat *handler.MovementHandler) {
	if mDat.TicksSinceKnockback != 0 {
		return
	}
	mDat.Velocity = mDat.Knockback
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

func (MovementSimulator) blockInside(mDat *handler.MovementHandler, w *world.World) (df_world.Block, bool) {
	bb := mDat.BoundingBox()
	for _, result := range utils.GetNearbyBlocks(bb, false, true, w) {
		pos := result.Position
		block := result.Block
		boxes := utils.BlockBoxes(block, pos, w)

		for _, box := range boxes {
			if bb.IntersectsWith(box.Translate(pos.Vec3())) {
				return block, true
			}
		}
	}

	return nil, false
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
		stepVel[1] -= rDy

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
	mDat.CollisionX = mDat.Velocity.X() != old.X()
	mDat.CollisionZ = mDat.Velocity.Z() != old.Z()
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
