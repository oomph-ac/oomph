package bedsim

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type mockWorld struct{}

func (mockWorld) Block(pos cube.Pos) world.Block {
	return block.Air{}
}

func (mockWorld) BlockCollisions(pos cube.Pos) []cube.BBox {
	return nil
}

func (mockWorld) GetNearbyBBoxes(aabb cube.BBox) []cube.BBox {
	return nil
}

func (mockWorld) IsChunkLoaded(chunkX, chunkZ int32) bool {
	return true
}

type mockEffects struct{}

func (mockEffects) GetEffect(effectID int32) (int32, bool) {
	return 0, false
}

func TestSimulateMoveRelative(t *testing.T) {
	sim := &Simulator{
		World:   mockWorld{},
		Effects: mockEffects{},
		Options: SimulationOptions{
			UseSlideOffset:              false,
			PositionCorrectionThreshold: 0.3,
		},
	}

	state := &MovementState{
		Pos:                  mgl64.Vec3{},
		Vel:                  mgl64.Vec3{},
		Mov:                  mgl64.Vec3{},
		Size:                 mgl64.Vec3{0.6, 1.8, 1},
		MovementSpeed:        0.1,
		DefaultMovementSpeed: 0.1,
		AirSpeed:             0.02,
		OnGround:             false,
		HasGravity:           true,
		Ready:                true,
		Alive:                true,
		GameMode:             packet.GameTypeSurvival,
		TicksSinceTeleport:   1,
	}

	input := InputState{
		MoveVector: mgl64.Vec2{0, 1},
		ClientPos:  mgl64.Vec3{},
		ClientVel:  mgl64.Vec3{},
		Yaw:        0,
		Pitch:      0,
		HeadYaw:    0,
	}

	result := sim.Simulate(state, input)
	if result.Velocity.Z() <= 0 {
		t.Fatalf("expected forward velocity, got %v", result.Velocity)
	}
	if result.Velocity.Y() >= 0 {
		t.Fatalf("expected gravity to apply, got %v", result.Velocity)
	}
}
