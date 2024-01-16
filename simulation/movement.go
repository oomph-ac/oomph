package simulation

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type MovementSimulator struct {
}

func (s MovementSimulator) Simulate(p *player.Player) {
	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	if !s.Reliable(p) {
		mDat.Velocity = mDat.ClientVel
		mDat.PrevVelocity = mDat.PrevClientVel
		mDat.Position = mDat.ClientPosition
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
	s.pushOutOfBlocks(p, mDat)

	// Reset the velocity to zero if it's significantly small.
	if mDat.Velocity.LenSqr() < 1e-8 {
		mDat.Velocity[0] = 0
		mDat.Velocity[1] = 0
		mDat.Velocity[2] = 0
	}

	// Apply knockback if applicable.
	s.knockback(mDat)

	mDat.StepClipOffset *= game.StepClipMultiplier
	if mDat.StepClipOffset < 1e-7 {
		mDat.StepClipOffset = 0
	}

	w := p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).World
	//blockUnder := w.GetBlock(cube.PosFromVec3(mDat.Position.Sub(mgl32.Vec3{0, 0.5})))

	blockFriction := game.DefaultAirFriction
	v3 := mDat.AirSpeed
	if mDat.OnGround {
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
}

func (MovementSimulator) Reliable(p *player.Player) bool {
	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	cDat := p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler)

	return (p.GameMode == packet.GameTypeSurvival || p.GameMode == packet.GameTypeAdventure) && !mDat.Flying &&
		!mDat.NoClip && p.Alive && cDat.InLoadedChunk
}

func (s MovementSimulator) teleport(p *player.Player, mDat *handler.MovementHandler) bool {
	if !mDat.SmoothTeleport && mDat.TicksSinceTeleport == 0 {
		mDat.Position = mDat.TeleportPos
		mDat.Velocity = mgl32.Vec3{}
		if mDat.OnGround {
			mDat.Velocity[1] = -0.002
		}

		mDat.TicksUntilNextJump = 0
		s.jump(mDat)
		return true
	} else if mDat.SmoothTeleport && mDat.TicksSinceTeleport <= 3 {
		mDat.Velocity[1] = -0.078
		delta := mDat.TeleportPos.Sub(mDat.Position)
		if mDat.TicksSinceTeleport < 3 {
			mDat.Position = mDat.Position.Add(delta.Mul(1.0 / float32(3-mDat.TicksSinceTeleport)))
			return true
		}

		mDat.Position = mDat.TeleportPos
		mDat.OnGround = mDat.TeleportOnGround
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

func (MovementSimulator) pushOutOfBlocks(p *player.Player, mDat *handler.MovementHandler) {
	if mDat.StepClipOffset > 0 {
		return
	}

	world := p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).World
	blockIn := world.GetBlock(cube.PosFromVec3(mDat.Position))
	if utils.CanPassBlock(blockIn) {
		mDat.KnownInsideBlock = false
		return
	}

	inside := false
	bb := mDat.BoundingBox
	newPos := mDat.Position

	airBlocks := map[cube.Face]bool{}
	for _, face := range cube.Faces() {
		facedBlockPos := cube.PosFromVec3(mDat.Position).Side(face)
		_, isAir := world.GetBlock(facedBlockPos).(block.Air)
		airBlocks[face] = isAir
	}

	for _, result := range utils.GetNearbyBlocks(mDat.BoundingBox, false, true, world) {
		if !utils.CanPassBlock(result.Block) {
			continue
		}

		for _, box := range utils.BlockBoxes(result.Block, result.Position, world) {
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
	v2, v3 := game.MCCos(force), game.MCSin(force)

	mDat.Velocity[0] += ms*v3 - mf*v2
	mDat.Velocity[2] += ms*v2 + mf*v3
}
