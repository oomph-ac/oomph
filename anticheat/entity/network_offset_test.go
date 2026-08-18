package entity

import "testing"

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
