# bedsim

`bedsim` is Oomph's extracted movement simulation library.

## What it provides
- Input + state simulation (`Simulator.Simulate` / `Simulator.SimulateState`)
- Collision resolution, stepping, edge-avoidance, glide, teleport handling
- Correction metadata (`PositionDelta`, `VelocityDelta`, `NeedsCorrection`, `Outcome`)

## Integration points
- `WorldProvider`: blocks, collisions, nearby boxes, chunk-loaded checks
- `EffectsProvider`: jump boost / levitation amplifiers
- `InventoryProvider`: Elytra checks for glide simulation

## Debug traces
Set `SimulationOptions.Debugf` to receive internal per-step trace logs (jump gating, collision passes, step success/failure, edge-avoid adjustments, etc.).  
Oomph wires this into `DebugModeMovementSim` in `player/simulation/bedsim_adapter.go`.
