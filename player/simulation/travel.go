package simulation

import (
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func doGlideMove(p *player.Player) {
	movement := p.Movement()
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
}

func doNormalMove(p *player.Player, clientJumpPrevented *bool) {
	movement := p.Movement()
	blockUnder := p.World().Block(df_cube.Pos(cube.PosFromVec3(movement.Pos().Sub(mgl32.Vec3{0, 0.5}))))
	blockFriction := game.DefaultAirFriction
	moveRelativeSpeed := movement.AirSpeed()
	if movement.OnGround() {
		blockFriction *= utils.BlockFriction(blockUnder)
		moveRelativeSpeed = movement.MovementSpeed() * (0.16277136 / (blockFriction * blockFriction * blockFriction))
	}
	movement.SetCurrentFriction(blockFriction)

	// Apply knockback if applicable.
	p.Dbg.Notify(player.DebugModeMovementSim, attemptKnockback(movement), "knockback applied: %v", movement.Vel())
	// Attempt jump velocity if applicable.
	p.Dbg.Notify(player.DebugModeMovementSim, attemptJump(movement, p.Dbg, clientJumpPrevented), "jump force applied (sprint=%v): %v", movement.Sprinting(), movement.Vel())

	p.Dbg.Notify(player.DebugModeMovementSim, true, "blockUnder=%s, blockFriction=%v, speed=%v", utils.BlockName(blockUnder), blockFriction, moveRelativeSpeed)
	moveRelative(movement, moveRelativeSpeed)
	p.Dbg.Notify(player.DebugModeMovementSim, true, "moveRelative force applied (vel=%v)", movement.Vel())
}

func livingEntityTravel(p *player.Player) {
	movement := p.Movement()
	clientJumpPrevented := false
	if movement.Gliding() {
		doGlideMove(p)
	} else {
		doNormalMove(p, &clientJumpPrevented)
		doNormalCollisions(p, clientJumpPrevented)
	}
}

func doPlayerTravel(p *player.Player) {
	prePlayerTravel(p)
	travelInAir(p, p.Movement().CurrentFriction()) // TODO: Check which end-tick scenario to apply for movement.
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
