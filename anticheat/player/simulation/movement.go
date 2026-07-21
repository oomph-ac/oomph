package simulation

import (
	"github.com/oomph-ac/bedsim"
	"github.com/oomph-ac/oomph/anticheat/game"
	"github.com/oomph-ac/oomph/anticheat/player"
)

// SimulatePlayerMovement runs a movement simulation tick for the player via bedsim.
func SimulatePlayerMovement(p *player.Player, movement player.MovementComponent) {
	if movement == nil {
		p.Disconnect(game.ErrorInternalMissingMovementComponent)
		return
	}

	p.Dbg.Notify(player.DebugModeMovementSim, true, "BEGIN movement sim for frame %d", p.SimulationFrame)
	defer p.Dbg.Notify(player.DebugModeMovementSim, true, "END movement sim for frame %d", p.SimulationFrame)

	p.Dbg.Notify(player.DebugModeMovementSim, true, "mF=%.4f, mS=%.4f", movement.Impulse().Y(), movement.Impulse().X())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "yaw=%.4f, pitch=%.4f", movement.Rotation().Z(), movement.Rotation().X())

	// Preserve Oomph's liquid exemption until swimming / second-layer liquid state is wired into
	// the bedsim adapter. bedsim itself can simulate liquids, but doing so without that state
	// would diverge from current authoritative behavior. Teleports still run through bedsim.
	if !movement.HasTeleport() && movement.RemainingTeleportTicks() <= 0 && intersectingLiquid(p, movement) {
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: unsupported scenario", p.SimulationFrame)
		movement.Reset()
		return
	}

	result := simulateWithBedsim(p, movement)
	switch result.Outcome {
	case bedsim.SimulationOutcomeTeleport:
		p.Dbg.Notify(player.DebugModeMovementSim, true, "teleport (newPos=%v)", movement.Pos())
		return
	case bedsim.SimulationOutcomeUnreliable:
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: unsupported scenario", p.SimulationFrame)
		return
	case bedsim.SimulationOutcomeUnloadedChunk:
		p.Dbg.Notify(player.DebugModeMovementSim, true, "no movement sim for frame %d: in unloaded chunk, cancelling all movement", p.SimulationFrame)
		return
	case bedsim.SimulationOutcomeImmobileOrNotReady:
		p.Dbg.Notify(player.DebugModeMovementSim, true, "player is immobile")
		return
	case bedsim.SimulationOutcomeNormal:
		// Handled below.
	default:
		p.Dbg.Notify(player.DebugModeMovementSim, true, "unexpected simulation outcome %v for frame %d", result.Outcome, p.SimulationFrame)
	}

	p.Dbg.Notify(player.DebugModeMovementSim, !movement.HasGravity(), "not affected by gravity?")
	p.Dbg.Notify(player.DebugModeMovementSim, true, "endOfFrameVel=%v", movement.Vel())
	p.Dbg.Notify(player.DebugModeMovementSim, true, "serverPos=%v clientPos=%v, diff=%v", movement.Pos(), movement.Client().Pos(), movement.Pos().Sub(movement.Client().Pos()))
}
