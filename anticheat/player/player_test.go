package player

import (
	"log/slog"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
)

type interactionPositionHistory struct {
	current, previous mgl32.Vec3
}

func (h interactionPositionHistory) Pos() mgl32.Vec3 {
	return h.current
}

func (h interactionPositionHistory) LastPos() mgl32.Vec3 {
	return h.previous
}

func TestBlockAddressWithoutRakNetListener(t *testing.T) {
	p := New(slog.Default(), MonitoringState{CurrentTime: time.Now()}, nil)
	p.BlockAddress(time.Second)
}

func TestInteractionBlockPositionsUsesPositionHistory(t *testing.T) {
	previous, current := interactionBlockPositions(interactionPositionHistory{
		previous: mgl32.Vec3{-1.1, 4, 2.9},
		current:  mgl32.Vec3{3.1, 8, -4.1},
	})

	if want := (cube.Pos{-2, 5, 2}); previous != want {
		t.Fatalf("previous position = %v, want %v", previous, want)
	}
	if want := (cube.Pos{3, 9, -5}); current != want {
		t.Fatalf("current position = %v, want %v", current, want)
	}
}
