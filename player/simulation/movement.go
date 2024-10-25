package simulation

import (
	"strings"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/assert"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// SimulatePlayerMovement is a function that runs a movement simulation for
func SimulatePlayerMovement(p *player.Player) {
	movement := p.Movement()
	assert.IsTrue(movement != nil, "movement component should be non-nil for simulation")

	// Check if we are in a loaded chunk, otherwise reset to the player's movement.
	if p.World.GetChunk(protocol.ChunkPos{
		int32(movement.Pos().X()) >> 4,
		int32(movement.Pos().Z()) >> 4,
	}) == nil {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: no available chunks", p.ClientFrame)
		movement.Reset()
		return
	} else if !simulationIsReliable(p) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: unsupported scenario", p.ClientFrame)
		movement.Reset()
		return
	}

	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN movement sim for frame %d", p.ClientFrame)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement sim for frame %d", p.ClientFrame)

	// If a teleport was able to be handled, do not continue with the simulation.
	if attemptTeleport(movement) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", movement.Pos())
		return
	}

	if movement.NoClientPredictions() || !p.Ready {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player is immobile")
		movement.SetVel(mgl32.Vec3{})
		return
	}

	// Reset the velocity to zero if it's significantly small.
	if movement.Vel().LenSqr() < 1e-12 {
		movement.SetVel(mgl32.Vec3{})
	}

	// Apply knockback if applicable.
	p.Dbg.Notify(player.DebugModeMovementSim, attemptKnockback(movement), "knockback applied: %v", movement.Vel())
	// Attempt jump velocity if applicable.
	p.Dbg.Notify(player.DebugModeMovementSim, attemptJump(movement), "jump force applied (sprint=%v): %v", movement.Sprinting(), movement.Vel())

	blockUnder := p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.5}))))
	blockFriction := game.DefaultAirFriction
	speed := movement.AirSpeed()

	if movement.OnGround() {
		blockFriction *= utils.BlockFriction(blockUnder)
		speed = movement.MovementSpeed() * (0.162771336 / math32.Pow(blockFriction, 3))
	}

	p.Dbg.Notify(player.DebugModeMovementSim, true, "blockUnder=%s, blockFriction=%v, speed=%v", utils.BlockName(blockUnder), blockFriction, speed)
	moveRelative(movement, speed)
	p.Dbg.Notify(player.DebugModeMovementSim, true, "moveRelative force applied (vel=%v)", movement.Vel())

	nearClimable := utils.BlockClimbable(p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()))))
	if nearClimable && !movement.PressingJump() {
		newVel := movement.Vel()
		newVel[0] = game.ClampFloat(newVel[0], -0.2, 0.2)
		newVel[2] = game.ClampFloat(newVel[2], -0.2, 0.2)

		if newVel[1] < -0.2 {
			newVel[1] = -0.2
		}
		if movement.Sneaking() && newVel[1] < 0 {
			newVel[1] = 0
		}

		movement.SetVel(newVel)
	}

	blocksInside, isInsideBlock := blocksInside(movement, p.World)
	inCobweb := false
	if isInsideBlock {
		for _, b := range blocksInside {
			if utils.BlockName(b) == "minecraft:web" {
				inCobweb = true
				break
			}
		}
	}

	if inCobweb {
		newVel := movement.Vel()
		newVel[0] *= 0.25
		newVel[1] *= 0.05
		newVel[2] *= 0.25
		movement.SetVel(newVel)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "cobweb force applied (vel=%v)", newVel)
	}

	// Avoid edges if the player is sneaking on the edge of a block.
	avoidEdge(movement, p.World)

	oldVel := movement.Vel()
	tryCollisions(movement, p.World, p.Dbg)
	movement.SetMov(movement.Vel())

	blockUnder = p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.2}))))
	if _, isAir := blockUnder.(block.Air); isAir {
		b := p.World.Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()).Side(cube.FaceDown)))
		n := utils.BlockName(b)
		if utils.IsFence(b) || utils.IsWall(n) || strings.Contains(n, "fence") {
			blockUnder = b
		}
	}

	isClimb := nearClimable
	setPostCollisionMotion(movement, oldVel, isClimb, blockUnder)
	walkOnBlock(movement, blockUnder)

	if inCobweb {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move cobweb force applied (0 vel)")
		movement.SetVel(mgl32.Vec3{})
	}

	newVel := movement.Vel()
	newVel[1] -= movement.Gravity()
	newVel[1] *= game.GravityMultiplier
	newVel[0] *= blockFriction
	newVel[2] *= blockFriction

	if isClimb {
		newVel[1] = game.ClimbSpeed
		p.Dbg.Notify(player.DebugModeMovementSim, true, "upward climb applied")
	}

	movement.SetVel(newVel)
	p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move final velocity: %v", newVel)
}

func walkOnBlock(movement player.MovementComponent, blockUnder df_world.Block) {
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

func simulationIsReliable(p *player.Player) bool {
	movement := p.Movement()
	if movement.RemainingTeleportTicks() > 0 {
		return true
	}

	for _, b := range utils.GetNearbyBlocks(movement.BoundingBox(), false, true, p.World) {
		if _, isLiquid := b.Block.(df_world.Liquid); isLiquid {
			return false
		}

		if utils.BlockName(b.Block) == "minecraft:bamboo" {
			return false
		}
	}

	return (p.GameMode == packet.GameTypeSurvival || p.GameMode == packet.GameTypeAdventure) &&
		!movement.Flying() &&
		!movement.NoClientPredictions() &&
		p.Alive &&
		movement.Pos().Y() >= -64
}

func landOnBlock(movement player.MovementComponent, old mgl32.Vec3, blockUnder df_world.Block) {
	newVel := movement.Vel()
	if movement.OnGround() || old.Y() >= 0 || movement.PressingSneak() {
		newVel[1] = 0
		movement.SetVel(newVel)
		return
	}

	switch utils.BlockName(blockUnder) {
	case "minecraft:slime":
		newVel[1] = game.SlimeBounceMultiplier * old.Y()
	case "minecraft:bed":
		newVel[1] = game.BedBounceMultiplier * old.Y()
	default:
		newVel[1] = 0
	}
	movement.SetVel(newVel)
}

func setPostCollisionMotion(movement player.MovementComponent, old mgl32.Vec3, climb bool, blockUnder df_world.Block) {
	if climb {
		return
	}

	if movement.YCollision() {
		landOnBlock(movement, old, blockUnder)
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

func tryCollisions(movement player.MovementComponent, w *world.World, dbg *player.Debugger) {
	currVel := movement.Vel()
	collisionBB := movement.BoundingBox()
	bbList := utils.GetNearbyBBoxes(collisionBB.Extend(currVel), w)
	oneWayBlocks := utils.OneWayCollisionBlocks(utils.GetNearbyBlocks(collisionBB.Extend(currVel), false, false, w))

	// TODO: determine more blocks that are considered to be one-way physics blocks, and
	// figure out how to calculate ActorCollision::isStuckItem()
	oneWayCollisions := len(oneWayBlocks) != 0 || movement.StuckInCollider()
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
	movement.SetStuckInCollider(movement.PenetratedLastFrame() && hasPenetration)
	movement.SetPenetratedLastFrame(hasPenetration)

	xCollision := currVel.X() != collisionVel.X()
	yCollision := currVel.Y() != collisionVel.Y()
	zCollision := currVel.Z() != collisionVel.Z()
	onGround := movement.OnGround() || (yCollision && currVel.Y() < 0.0)

	if onGround && (xCollision || zCollision) {
		v4 := mgl32.Vec3{0, game.StepHeight}
		v5 := mgl32.Vec3{currVel.X()}
		v6 := mgl32.Vec3{0, 0, currVel.Z()}

		stepBB := movement.BoundingBox()
		for _, blockBox := range bbList {
			v4 = game.BBClipCollide(blockBox, stepBB, v4, oneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(v4)

		for _, blockBox := range bbList {
			v5 = game.BBClipCollide(blockBox, stepBB, v5, oneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(v5)

		for _, blockBox := range bbList {
			v6 = game.BBClipCollide(blockBox, stepBB, v6, oneWayCollisions, nil)
		}
		stepBB = stepBB.Translate(v6)

		v175 := v4.Mul(-1)
		for _, blockBox := range bbList {
			v175 = game.BBClipCollide(blockBox, stepBB, v175, oneWayCollisions, nil)
		}
		v4 = v4.Add(v175)

		finalVel := v4.Add(v6).Add(v5)
		newBBList := utils.GetNearbyBBoxes(stepBB, w)
		dbg.Notify(player.DebugModeMovementSim, true, "newBBList count: %d", len(newBBList))

		if len(newBBList) == 0 && game.Vec3HzDistSqr(collisionVel) < game.Vec3HzDistSqr(finalVel) {
			collisionVel = finalVel
			dbg.Notify(player.DebugModeMovementSim, true, "step successful: %v", collisionVel)
		}
	}

	movement.SetCollisions(
		math32.Abs(currVel.X()-collisionVel.X()) >= 1e-5, // xCollision
		math32.Abs(currVel.Y()-collisionVel.Y()) >= 1e-5, // yCollision
		math32.Abs(currVel.Z()-collisionVel.Z()) >= 1e-5, // zCollision
	)
	movement.SetOnGround((movement.YCollision() && currVel.Y() < 0) || (movement.OnGround() && !movement.YCollision() && currVel.Y() == 0.0))
	movement.SetVel(collisionVel)
	movement.SetPos(movement.Pos().Add(collisionVel))
	dbg.Notify(player.DebugModeMovementSim, true, "finalVel=%v finalPos=%v", collisionVel, movement.Pos())
}

func avoidEdge(movement player.MovementComponent, w *world.World) {
	if !movement.Sneaking() || !movement.OnGround() || movement.Vel().Y() > 0 {
		return
	}

	newVel := movement.Vel()
	bb := movement.BoundingBox().GrowVec3(mgl32.Vec3{-0.025, 0, -0.025})
	xMov, zMov, offset := newVel.X(), newVel.Z(), float32(0.05)

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

	newVel[0] = xMov
	newVel[2] = zMov
	movement.SetVel(newVel)
}

func blocksInside(movement player.MovementComponent, w *world.World) ([]df_world.Block, bool) {
	bb := movement.BoundingBox()
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

func moveRelative(movement player.MovementComponent, speed float32) {
	force := math32.Pow(movement.Impulse().Y(), 2) + math32.Pow(movement.Impulse().X(), 2)
	if force < 1e-4 {
		return
	}

	force = math32.Sqrt(force)
	if force < 1 {
		force = 1
	}
	force = speed / force

	mf, ms := movement.Impulse().Y()*force, movement.Impulse().X()*force
	v1 := movement.Rotation().Z() * (math32.Pi / 180.0)
	v2, v3 := game.MCSin(v1), game.MCCos(v1)

	newVel := movement.Vel()
	newVel[0] += ms*v3 - mf*v2
	newVel[2] += ms*v2 + mf*v3
	movement.SetVel(newVel)
}

func attemptKnockback(movement player.MovementComponent) bool {
	if movement.HasKnockback() {
		movement.SetVel(movement.Knockback())
		return true
	}
	return false
}

func attemptJump(movement player.MovementComponent) bool {
	if !movement.Jumping() || !movement.OnGround() || movement.JumpDelay() > 0 {
		return false
	}

	newVel := movement.Vel()
	newVel[1] = movement.JumpHeight()
	movement.SetJumpDelay(game.JumpDelayTicks)

	if movement.Sprinting() {
		force := movement.Rotation().Z() * 0.017453292
		newVel[0] -= game.MCSin(force) * 0.2
		newVel[2] += game.MCCos(force) * 0.2
	}

	movement.SetVel(newVel)
	return true
}

func attemptTeleport(movement player.MovementComponent) bool {
	if !movement.HasTeleport() {
		return false
	}

	if !movement.TeleportSmoothed() {
		movement.SetPos(movement.TeleportPos())
		newVel := mgl32.Vec3{}
		if movement.OnGround() {
			newVel[1] -= 0.002
		}

		movement.SetVel(newVel)
		movement.SetJumpDelay(0)
		attemptJump(movement)
	} else {
		posDelta := movement.TeleportPos().Sub(movement.Pos())
		movement.SetVel(mgl32.Vec3{0, -0.078})
		if remaining := movement.RemainingTeleportTicks(); remaining > 1 {
			newPos := movement.Pos().Add(posDelta.Mul(1.0 / float32(remaining)))
			movement.SetPos(newPos)
		}
	}

	return true
}
