package simulation

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// SimulatePlayerMovement is a function that runs a movement simulation for
func SimulatePlayerMovement(p *player.Player, movement player.MovementComponent) {
	if movement == nil {
		p.Disconnect(game.ErrorInternalMissingMovementComponent)
		return
	}

	//assert.IsTrue(movement != nil, "movement component should be non-nil for simulation")

	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN movement sim for frame %d", p.SimulationFrame)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement sim for frame %d", p.SimulationFrame)

	p.Dbg.Notify(player.DebugModeMovementSim, true, "mF=%.4f, mS=%.4f", movement.Impulse().Y(), movement.Impulse().X())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "yaw=%.4f, pitch=%.4f", movement.Rotation().Z(), movement.Rotation().X())

	if !simulationIsReliable(p, movement) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: unsupported scenario", p.SimulationFrame)
		movement.Reset()
		return
	} else if p.World().GetChunk(protocol.ChunkPos{int32(movement.Pos().X()) >> 4, int32(movement.Pos().Z()) >> 4}) == nil {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: in unloaded chunk, cancelling all movement", p.SimulationFrame)
		movement.SetVel(mgl32.Vec3{})
		return
	}

	// If a teleport was able to be handled, do not continue with the simulation.
	if attemptTeleport(movement, p.Dbg) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", movement.Pos())
		/* if attemptKnockback(movement) {
			p.Dbg.Notify(player.DebugModeMovementSim, true, "knockback applied: %v", movement.Vel())
			movement.SetPos(movement.Pos().Add(movement.Vel()))
		} */
		return
	}

	if movement.Immobile() || !p.Ready {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player is immobile")
		movement.SetVel(mgl32.Vec3{})
		return
	}
	if movement.Vel().LenSqr() < 1e-12 { // Reset the velocity to zero if it's significantly small.
		movement.SetVel(mgl32.Vec3{})
	}
	livingEntityTravel(p)
}

func simulateGlide(p *player.Player, movement player.MovementComponent) {
	radians := (math32.Pi / 180.0)
	yaw, pitch := movement.Rotation().Z()*radians, movement.Rotation().X()*radians
	yawCos := game.MCCos(-yaw - math32.Pi)
	yawSin := game.MCSin(-yaw - math32.Pi)
	pitchCos := game.MCCos(pitch)
	pitchSin := game.MCSin(pitch)

	lookX := yawSin * -pitchCos
	lookY := -pitchSin
	lookZ := yawCos * -pitchCos

	vel := movement.Vel()
	velHz := math32.Sqrt(vel[0]*vel[0] + vel[2]*vel[2])
	lookHz := pitchCos
	sqrPitchCos := pitchCos * pitchCos

	vel[1] += -0.08 + sqrPitchCos*0.06
	if vel[1] < 0 && lookHz > 0 {
		yAccel := vel[1] * -0.1 * sqrPitchCos
		vel[1] += yAccel
		vel[0] += lookX * yAccel / lookHz
		vel[2] += lookZ * yAccel / lookHz
	}
	if pitch < 0 {
		yAccel := velHz * -pitchSin * 0.04
		vel[1] += yAccel * 3.2
		vel[0] -= lookX * yAccel / lookHz
		vel[2] -= lookZ * yAccel / lookHz
	}
	if lookHz > 0 {
		vel[0] += (lookX/lookHz*velHz - vel[0]) * 0.1
		vel[2] += (lookZ/lookHz*velHz - vel[2]) * 0.1
	}

	// Although this should be applied when a fireworks entity is ticked (BECAUSE THIS IS WHAT EVERY SINGLE DECOMPILATION OF EVERY SINGLE
	// VERSION OF BOTH MCBE AND MCJE SHOWS), putting the logic here allowed Oomph's prediction to be accurate...
	// Furthermore, it seems that the client only likes to have one active boost at a time (although this seems to defy)
	// the logic provided by *EVERY SINGLE DECOMPILATION OF EVERY SINGLE VERSION OF BOTH MCBE AND MCJE*
	// Spending too many hours on stupid [sugar honey iced tea] like this better be making me bank in the near future.
	if movement.GlideBoost() > 0 {
		oldVel := vel
		vel[0] += (lookX * 0.1) + (((lookX * 1.5) - vel[0]) * 0.5)
		vel[1] += (lookY * 0.1) + (((lookY * 1.5) - vel[1]) * 0.5)
		vel[2] += (lookZ * 0.1) + (((lookZ * 1.5) - vel[2]) * 0.5)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "applied glide boost (old=%v new=%v)", oldVel, vel)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "glide boost dirVec=[%f %f %f]", lookX, lookY, lookZ)
	}

	vel[0] *= 0.99
	vel[1] *= 0.98
	vel[2] *= 0.99

	movement.SetVel(vel)

	oldVel := vel
	runCollisions(movement, p.World(), p.Dbg, p.VersionInRange(-1, player.GameVersion1_20_60), false)
	velDiff := movement.Vel().Sub(movement.Client().Vel())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "(glide) oldVel=%v, collisions=%v diff=%v", oldVel, movement.Vel(), velDiff)
}

func walkOnBlock(movement player.MovementComponent, blockUnder world.Block) {
	if !movement.OnGround() || movement.Sneaking() {
		return
	}

	newVel := movement.Vel()
	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		yMov := math32.Abs(newVel.Y())
		if yMov < 0.1 && !movement.PressingSneak() {
			d1 := 0.4 + yMov*0.2
			newVel[0] *= d1
			newVel[2] *= d1
		}
	case "minecraft:soul_sand":
		newVel[0] *= 0.3
		newVel[2] *= 0.3
	}
	movement.SetVel(newVel)
}

func simulationIsReliable(p *player.Player, movement player.MovementComponent) bool {
	if movement.RemainingTeleportTicks() > 0 {
		return true
	}

	for _, b := range utils.GetNearbyBlocks(movement.BoundingBox().Grow(1), false, true, p.World()) {
		/* if _, isLiquid := b.Block.(world.Liquid); isLiquid {
			blockBB := cube.Box(0, 0, 0, 1, 1, 1).Translate(b.Position.Vec3())
			if movement.BoundingBox().IntersectsWith(blockBB) {
				return false
			}
		} */
		if utils.BlockName(b.Block) == "minecraft:bamboo" {
			return false
		}
	}

	return (p.GameMode == packet.GameTypeSurvival || p.GameMode == packet.GameTypeAdventure) &&
		!(movement.Flying() || movement.NoClip() || !p.Alive)
}

func landOnBlock(movement player.MovementComponent, old mgl32.Vec3, blockUnder world.Block) {
	newVel := movement.Vel()
	if old.Y() >= 0 || movement.PressingSneak() {
		newVel[1] = 0
		movement.SetVel(newVel)
		return
	}

	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		newVel[1] = game.SlimeBounceMultiplier * old.Y()
	case "minecraft:bed":
		newVel[1] = math32.Min(1.0, game.BedBounceMultiplier*old.Y())
	default:
		newVel[1] = 0
	}
	movement.SetVel(newVel)
}

func applyPostCollisionVelocity(p *player.Player, oldVel mgl32.Vec3, oldOnGround bool, blockUnder world.Block) {
	movement := p.Movement()
	if !movement.InWater() {
		updateInWaterStateAndDoWaterCurrentPushing(p)
	}

	if !oldOnGround && movement.YCollision() {
		landOnBlock(movement, oldVel, blockUnder)
	} else if movement.YCollision() {
		newVel := movement.Vel()
		newVel[1] = 0
		movement.SetVel(newVel)
	}

	newVel := movement.Vel()
	if movement.XCollision() {
		newVel[0] = 0
	}
	if movement.ZCollision() {
		newVel[2] = 0
	}
	movement.SetVel(newVel)
}

// avoidEdge is the function that helps the movement component remain at the edge of a block when sneaking.
func avoidEdge(movement player.MovementComponent, src world.BlockSource, dbg *player.Debugger) {
	if !movement.Sneaking() || !movement.OnGround() || movement.Vel().Y() > 0 {
		dbg.Notify(
			player.DebugModeMovementSim,
			true,
			"avoidEdge: conditions not met (sneaking=%v onGround=%v yVel=%v)",
			movement.Sneaking(),
			movement.OnGround(),
			movement.Vel().Y(),
		)
		return
	}

	var (
		// Unlike in MCJE, where the edge boundry is 0.03, looking through a decomplilation of MCBE's 1.16 China
		// binary shows that on Bedrock the edge boundry is 0.025 on the X and Z axis.
		edgeBoundry float32 = 0.025
		offset      float32 = 0.05
	)

	newVel := movement.Vel()
	bb := movement.BoundingBox().GrowVec3(mgl32.Vec3{-edgeBoundry, 0, -edgeBoundry})
	xMov, zMov := newVel.X(), newVel.Z()

	for xMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, 0}), src)) == 0 {
		if xMov < offset && xMov >= -offset {
			xMov = 0
		} else if xMov > 0 {
			xMov -= offset
		} else {
			xMov += offset
		}
	}

	for zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{0, -game.StepHeight * 1.01, zMov}), src)) == 0 {
		if zMov < offset && zMov >= -offset {
			zMov = 0
		} else if zMov > 0 {
			zMov -= offset
		} else {
			zMov += offset
		}
	}

	for xMov != 0.0 && zMov != 0.0 && len(utils.GetNearbyBBoxes(bb.Translate(mgl32.Vec3{xMov, -game.StepHeight * 1.01, zMov}), src)) == 0 {
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

	oldVel := movement.Vel()
	newVel[0] = xMov
	newVel[2] = zMov
	movement.SetVel(newVel)

	dbg.Notify(player.DebugModeMovementSim, true, "(avoidEdge): oldVel=%v newVel=%v", oldVel, newVel)
}

func blocksInside(movement player.MovementComponent, src world.BlockSource) ([]world.Block, bool) {
	bb := movement.BoundingBox()
	blocks := []world.Block{}

	for _, result := range utils.GetNearbyBlocks(bb.Grow(1), false, true, src) {
		pos := result.Position
		block := result.Block
		boxes := utils.BlockBoxes(block, pos, src)

		for _, box := range boxes {
			if bb.IntersectsWith(box.Translate(pos.Vec3())) {
				blocks = append(blocks, block)
			}
		}
	}
	return blocks, len(blocks) > 0
}

func moveRelative(movement player.MovementComponent, moveRelativeSpeed float32) {
	impulse := movement.Impulse()
	force := impulse.Y()*impulse.Y() + impulse.X()*impulse.X()

	if force >= 1e-4 {
		force = moveRelativeSpeed / math32.Max(math32.Sqrt(force), 1.0)
		mf, ms := impulse.Y()*force, impulse.X()*force

		yaw := movement.Rotation().Z() * math32.Pi / 180.0
		v2, v3 := game.MCSin(yaw), game.MCCos(yaw)

		newVel := movement.Vel()
		newVel[0] += ms*v3 - mf*v2
		newVel[2] += mf*v3 + ms*v2
		movement.SetVel(newVel)
	}
}

func attemptKnockback(movement player.MovementComponent) bool {
	if movement.HasKnockback() {
		movement.SetVel(movement.Knockback())
		return true
	}
	return false
}

func attemptJump(movement player.MovementComponent, dbg *player.Debugger, clientJumpPrevented *bool) bool {
	if !movement.Jumping() || !movement.OnGround() || movement.JumpDelay() > 0 {
		dbg.Notify(player.DebugModeMovementSim, movement.Jumping(), "rejected jump from client (onGround=%v jumpDelay=%d)", movement.OnGround(), movement.JumpDelay())
		return false
	}

	// FIXME: The client seems to sometimes prevent it's own jump from happening - it is unclear how it is determined, however.
	// This is a temporary hack to get around this issue for now.
	clientJump := movement.Client().Pos().Y() - movement.Client().LastPos().Y()
	if math32.Abs(clientJump) <= 1e-4 && !movement.HasKnockback() && !movement.HasTeleport() && clientJumpPrevented != nil {
		*clientJumpPrevented = true
	}

	newVel := movement.Vel()
	newVel[1] = math32.Max(movement.JumpHeight(), newVel[1])
	movement.SetJumpDelay(game.JumpDelayTicks)

	if movement.Sprinting() {
		force := movement.Rotation().Z() * 0.017453292
		newVel[0] -= game.MCSin(force) * 0.2
		newVel[2] += game.MCCos(force) * 0.2
	}

	movement.SetVel(newVel)
	return true
}

func attemptTeleport(movement player.MovementComponent, dbg *player.Debugger) bool {
	if !movement.HasTeleport() {
		return false
	}

	if !movement.TeleportSmoothed() {
		movement.SetPos(movement.TeleportPos())
		movement.SetVel(mgl32.Vec3{})
		movement.SetJumpDelay(0)
		attemptJump(movement, dbg, nil)
		return true
	}
	// Calculate the smooth teleport's next position.
	posDelta := movement.TeleportPos().Sub(movement.Pos())
	if remaining := movement.RemainingTeleportTicks() + 1; remaining > 0 {
		newPos := movement.Pos().Add(posDelta.Mul(1.0 / float32(remaining)))
		movement.SetPos(newPos)
		//movement.SetVel(mgl32.Vec3{})
		movement.SetJumpDelay(0)
		return remaining > 1
	}
	return false
}
