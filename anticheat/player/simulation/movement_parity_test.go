package simulation_test

import (
	"log/slog"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component"
	"github.com/oomph-ac/oomph/anticheat/player/simulation"
	oomphworld "github.com/oomph-ac/oomph/anticheat/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestNoClipResetPreservesOnGround(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{}, nil)
	movement := component.NewAuthoritativeMovementComponent(p)
	movement.ResetTransferState(mgl32.Vec3{})
	movement.SetNoClip(true)
	movement.SetOnGround(true)

	simulation.SimulatePlayerMovement(p, movement)

	if !movement.OnGround() {
		t.Fatal("NoClip reset cleared OnGround; want stable Oomph reset semantics")
	}
}

func TestLiquidMovementRunsThroughBedsim(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{}, nil)
	p.Ready = true
	p.Alive = true

	pos := mgl32.Vec3{0.5, 1, 0.5}
	movement := component.NewAuthoritativeMovementComponent(p)
	movement.ResetTransferState(pos)
	movement.SetSize(mgl32.Vec3{0.6, 1.8, 1})

	c := chunk.New(oomphworld.BlockRegistry, cube.Range(world.Overworld.Range()))
	c.SetBlock(0, 1, 0, 0, oomphworld.BlockRegistry.BlockRuntimeID(block.Water{Depth: 8}))
	p.World().AddChunk(protocol.ChunkPos{}, oomphworld.ChunkInfo{Chunk: c})

	simulation.SimulatePlayerMovement(p, movement)

	if movement.SwimWaterGraceTicks() == 0 {
		t.Fatal("liquid movement was reset instead of simulated by bedsim")
	}
}
