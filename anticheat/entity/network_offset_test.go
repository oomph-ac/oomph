package entity

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func TestNetworkOffsetPlayerPoses(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[uint32]any
		want     float32
	}{
		{name: "standing", want: 1.62001},
		{name: "sleeping", metadata: map[uint32]any{DataKeyPlayerFlags: byte(1 << DataPlayerFlagSleep)}, want: 0.2},
		{name: "sneaking", metadata: map[uint32]any{DataKeyFlags: int64(1 << DataFlagSneaking)}, want: 1.27001},
		{name: "swimming", metadata: map[uint32]any{DataKeyFlags: int64(1 << DataFlagSwimming)}, want: 0.4},
		{name: "gliding", metadata: map[uint32]any{DataKeyFlags: int64(1 << DataFlagGliding)}, want: 0.4},
		{name: "spin attack", metadata: map[uint32]any{DataKeyFlags: int64(1 << DataFlagSpinAttack)}, want: 0.4},
		{name: "crawling", metadata: map[uint32]any{DataKeyFlagsTwo: int64(1 << (DataFlagCrawling % 64))}, want: 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := networkOffset(TypePlayer, tt.metadata); got != tt.want {
				t.Fatalf("network offset = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityUpdateMetadataRefreshesNetworkOffset(t *testing.T) {
	e := New(Config{
		Type:            TypePlayer,
		Metadata:        map[uint32]any{},
		NetworkPosition: mgl32.Vec3{0, 100, 0},
		HistorySize:     4,
		IsPlayer:        true,
	})

	if e.NetworkOffset != 1.62001 {
		t.Fatalf("initial network offset = %v, want %v", e.NetworkOffset, float32(1.62001))
	}
	e.UpdateMetadata(map[uint32]any{DataKeyFlags: int64(1 << DataFlagSneaking)})
	if e.NetworkOffset != 1.27001 {
		t.Fatalf("sneaking network offset = %v, want %v", e.NetworkOffset, float32(1.27001))
	}
	e.UpdateMetadata(map[uint32]any{DataKeyFlags: int64(1 << DataFlagSwimming)})
	if e.NetworkOffset != 0.4 {
		t.Fatalf("swimming network offset = %v, want %v", e.NetworkOffset, float32(0.4))
	}
}
