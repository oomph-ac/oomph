package simulation

import (
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	oomph_block "github.com/oomph-ac/oomph/world/block"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func livingEntityTravel(p *player.Player) {
	movement := p.Movement()
	if movement.Gliding() {
		_, hasElytra := p.Inventory().Chestplate().Item().(item.Elytra)
		if hasElytra && !movement.OnGround() {
			movement.SetOnGround(false)
			simulateGlide(p, movement)
			movement.SetMov(movement.Vel())
		} else {
			if movement.OnGround() {
				movement.SetGliding(false)
			}
			p.Dbg.Notify(player.DebugModeMovementSim, true, "client wants glide, but has no elytra (or is on-ground) - forcing normal movement")
		}
	} else {
		clientJumpPrevented := false
		blockUnder := p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.5}))))
		blockFriction := game.DefaultAirFriction
		moveRelativeSpeed := movement.AirSpeed()
		if movement.OnGround() {
			blockFriction *= utils.BlockFriction(blockUnder)
			moveRelativeSpeed = movement.MovementSpeed() * (0.16277136 / (blockFriction * blockFriction * blockFriction))
		}

		// Apply knockback if applicable.
		p.Dbg.Notify(player.DebugModeMovementSim, attemptKnockback(movement), "knockback applied: %v", movement.Vel())
		// Attempt jump velocity if applicable.
		p.Dbg.Notify(player.DebugModeMovementSim, attemptJump(movement, p.Dbg, &clientJumpPrevented), "jump force applied (sprint=%v): %v", movement.Sprinting(), movement.Vel())

		p.Dbg.Notify(player.DebugModeMovementSim, true, "blockUnder=%s, blockFriction=%v, speed=%v", utils.BlockName(blockUnder), blockFriction, moveRelativeSpeed)
		moveRelative(movement, moveRelativeSpeed)
		p.Dbg.Notify(player.DebugModeMovementSim, true, "moveRelative force applied (vel=%v)", movement.Vel())

		nearClimbable := utils.BlockClimbable(p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()))))
		if nearClimbable {
			newVel := movement.Vel()
			//newVel[0] = game.ClampFloat(newVel[0], -0.3, 0.3)
			//newVel[2] = game.ClampFloat(newVel[2], -0.3, 0.3)

			negClimbSpeed := -game.ClimbSpeed
			if newVel[1] < negClimbSpeed {
				newVel[1] = negClimbSpeed
			}
			if movement.PressingJump() || movement.XCollision() || movement.ZCollision() {
				newVel[1] = game.ClimbSpeed
			}
			if movement.Sneaking() && newVel[1] < 0 {
				newVel[1] = 0
			}

			p.Dbg.Notify(player.DebugModeMovementSim, true, "added climb velocity: %v (collided=%v pressingJump=%v)", newVel, movement.XCollision() || movement.ZCollision(), movement.PressingJump())
			movement.SetVel(newVel)
		}

		blocksInside, isInsideBlock := blocksInside(movement, p.World())
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
		avoidEdge(movement, p.World(), p.Dbg)

		oldVel := movement.Vel()
		oldOnGround := movement.OnGround()

		tryCollisions(movement, p.World(), p.Dbg, p.VersionInRange(-1, player.GameVersion1_20_60), clientJumpPrevented)
		walkOnBlock(movement, blockUnder)
		movement.SetMov(movement.Vel())

		blockUnder = p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.2}))))
		if _, isAir := blockUnder.(block.Air); isAir {
			b := p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos()).Side(cube.FaceDown)))
			if oomph_block.IsWall(b) || oomph_block.IsFence(b) {
				blockUnder = b
			}
		}
		applyPostCollisionVelocity(p, oldVel, oldOnGround, blockUnder)
		if inCobweb {
			p.Dbg.Notify(player.DebugModeMovementSim, true, "post-move cobweb force applied (0 vel)")
			movement.SetVel(mgl32.Vec3{})
		}
		doPlayerTravel(p, blockFriction)
	}
}

func doPlayerTravel(p *player.Player, friction float32) {
	prePlayerTravel(p)
	travelInAir(p, friction) // TODO: Check which end-tick scenario to apply for movement.
}

func prePlayerTravel(p *player.Player) {
	// TODO: Implement vehicle logic in liquids.
	movement := p.Movement()
	if !movement.Swimming() {
		return
	}
	pitch := game.DirectionVector(movement.Rotation().Z(), movement.Rotation().X())[1]
	var multiplier float32
	if pitch < -0.2 {
		multiplier = 0.085
	} else {
		multiplier = 0.06
	}

	playerPos := p.Position()
	_, noLiquid := p.World().Block(df_cube.Pos{int(playerPos.X()), int(playerPos.Y() + 0.9), int(playerPos.Z())}).(world.Liquid)
	if pitch <= 0.0 || movement.PressingJump() || noLiquid {
		vel := movement.Vel()
		vel[1] += (pitch - vel[1]) * multiplier
		movement.SetVel(vel)
	}
}

func travelInAir(p *player.Player, friction float32) {
	movement := p.Movement()
	newVel := movement.Vel()
	if eff, ok := p.Effects().Get(packet.EffectLevitation); ok {
		levSpeed := game.LevitationGravityMultiplier * float32(eff.Amplifier)
		newVel[1] += (levSpeed - newVel[1]) * 0.2
	} else {
		newVel[1] -= movement.Gravity()
		newVel[1] *= game.NormalGravityMultiplier
	}
	newVel[0] *= friction
	newVel[2] *= friction

	movement.SetVel(newVel)
	p.Dbg.Notify(player.DebugModeMovementSim, true, "endOfFrameVel=%v", newVel)
	p.Dbg.Notify(player.DebugModeMovementSim, true, "serverPos=%v clientPos=%v, diff=%v", movement.Pos(), movement.Client().Pos(), movement.Pos().Sub(movement.Client().Pos()))
}
