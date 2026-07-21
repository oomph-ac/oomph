package component

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type liquidInputUpdater interface {
	updateLiquidInput(input protocol.Bitset)
	Swimming() bool
	SwimAmount() float32
	AutoJumpingInWater() bool
	WantDown() bool
	WantDownSlow() bool
	AscendBlock() bool
}

func TestTransferResetClearsLiquidState(t *testing.T) {
	movement := NewAuthoritativeMovementComponent(nil)

	start := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	start.Set(packet.InputFlagStartSwimming)
	movement.updateLiquidInput(start)

	liquidInput := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	liquidInput.Set(packet.InputFlagAutoJumpingInWater)
	liquidInput.Set(packet.InputFlagWantDown)
	liquidInput.Set(packet.InputFlagWantDownSlow)
	liquidInput.Set(packet.InputFlagAscendBlock)
	movement.updateLiquidInput(liquidInput)
	movement.SetSwimWaterGraceTicks(5)

	movement.ResetTransferState(mgl32.Vec3{})

	if movement.Swimming() || movement.SwimAmount() != 0 || movement.SwimWaterGraceTicks() != 0 {
		t.Fatalf("retained swimming state after transfer reset: swimming=%t amount=%v grace=%d",
			movement.Swimming(), movement.SwimAmount(), movement.SwimWaterGraceTicks())
	}
	if movement.AutoJumpingInWater() || movement.WantDown() || movement.WantDownSlow() || movement.AscendBlock() {
		t.Fatal("retained per-tick liquid input after transfer reset")
	}
}

func TestLiquidInputState(t *testing.T) {
	movement := &AuthoritativeMovementComponent{}
	updater, ok := any(movement).(liquidInputUpdater)
	if !ok {
		t.Fatal("movement component does not track liquid input")
	}

	start := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	start.Set(packet.InputFlagStartSwimming)
	start.Set(packet.InputFlagAutoJumpingInWater)
	start.Set(packet.InputFlagWantDown)
	start.Set(packet.InputFlagAscendBlock)
	updater.updateLiquidInput(start)
	if !updater.Swimming() || updater.SwimAmount() != 0 {
		t.Fatalf("start state = swimming %t, amount %v; want true, 0", updater.Swimming(), updater.SwimAmount())
	}
	if !updater.AutoJumpingInWater() || !updater.WantDown() || !updater.AscendBlock() {
		t.Fatal("current liquid input flags were not retained")
	}

	hold := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	updater.updateLiquidInput(hold)
	if !updater.Swimming() || updater.SwimAmount() != 0.1 {
		t.Fatalf("held state = swimming %t, amount %v; want true, 0.1", updater.Swimming(), updater.SwimAmount())
	}
	if updater.AutoJumpingInWater() || updater.WantDown() || updater.AscendBlock() {
		t.Fatal("one-tick liquid input flags were not cleared")
	}

	stop := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	stop.Set(packet.InputFlagStopSwimming)
	stop.Set(packet.InputFlagWantDownSlow)
	updater.updateLiquidInput(stop)
	if updater.Swimming() || updater.SwimAmount() != 0.2 {
		t.Fatalf("stop state = swimming %t, amount %v; want false, 0.2", updater.Swimming(), updater.SwimAmount())
	}
	if !updater.WantDownSlow() {
		t.Fatal("want-down-slow input was not retained")
	}
}
