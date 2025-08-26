package movement

import (
	"github.com/chewxy/math32"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Simulate is a function that runs a movement simulation for
func Simulate(p *player.Player, movement player.MovementComponent) {
	if movement == nil {
		p.Disconnect(game.ErrorInternalMissingMovementComponent)
		return
	}

	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN movement sim for frame %d", p.SimulationFrame)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement sim for frame %d", p.SimulationFrame)

	p.Dbg.Notify(player.DebugModeMovementSim, true, "mF=%.4f, mS=%.4f", movement.Impulse().Y(), movement.Impulse().X())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "yaw=%.4f, pitch=%.4f", movement.Rotation().Z(), movement.Rotation().X())

	ctx := newCtx(p)
	defer putCtx(ctx)

	ctx.updateInWaterStateAndDoFluidPushing()
	ctx.updateFluidOnEyes()
	p.Message("waterHeight=%.4f, lavaHeight=%.4f wasTouchingWater=%t (%d)", movement.WaterHeight(), movement.LavaHeight(), movement.WasTouchingWater(), p.InputCount)

	// ALWAYS simulate the teleport, as the client will always have the same behavior regardless of if the scenario
	// is "unreliable", or if the player currently is in an unloaded chunk.
	if ctx.tryTeleport() {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", movement.Pos())
		return
	}

	if !simulationIsReliable(p, movement) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: unsupported scenario", p.SimulationFrame)
		movement.Reset()
		return
	} else if p.World().GetChunk(protocol.ChunkPos{int32(movement.Pos().X()) >> 4, int32(movement.Pos().Z()) >> 4}) == nil {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: in unloaded chunk, cancelling all movement", p.SimulationFrame)
		movement.SetVel(mgl32.Vec3{})
		return
	}

	if movement.Immobile() || !p.Ready {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player is immobile")
		movement.SetVel(mgl32.Vec3{})
		return
	}

	// Reset the velocity to zero if it's significantly small.
	initVel := movement.Vel()
	if math32.Abs(initVel[0]) < 1e-6 {
		initVel[0] = 0
	}
	if math32.Abs(initVel[1]) < 1e-6 {
		initVel[1] = 0
	}
	if math32.Abs(initVel[2]) < 1e-6 {
		initVel[2] = 0
	}
	movement.SetVel(initVel)
	ctx.travel()
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

func blockPosAffectingMovement(pos mgl32.Vec3) cube.Pos {
	return cube.PosFromVec3(pos).Side(cube.FaceDown)
}
