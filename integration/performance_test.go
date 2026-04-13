package integration_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func BenchmarkEntityRewindExactAndNearest(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := entity.New(
		1,
		"minecraft:player",
		map[uint32]any{},
		mgl32.Vec3{},
		256,
		true,
		0.6,
		1.8,
		1.0,
		&logger,
	)

	for tick := int64(1); tick <= 256; tick++ {
		pos := mgl32.Vec3{float32(tick), 64, float32(tick)}
		if err := e.UpdatePosition(entity.HistoricalPosition{
			Position:     pos,
			PrevPosition: pos,
			Tick:         tick,
		}); err != nil {
			b.Fatalf("seed history failed: %v", err)
		}
	}

	targetTicks := []int64{16, 64, 127, 192, 255, 300}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		target := targetTicks[i%len(targetTicks)]
		if _, ok := e.Rewind(target); !ok {
			b.Fatalf("rewind returned !ok for tick=%d", target)
		}
	}
}

func BenchmarkKeyValsToString(b *testing.B) {
	kv := []any{
		"player", "bench_user",
		"detection", "Reach_A",
		"violations", 7.25,
		"latency", "42ms",
		"input_mode", "touch",
		"tick", int64(123456),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := utils.KeyValsToString(kv)
		if len(s) == 0 {
			b.Fatal("unexpected empty string")
		}
	}
}

func BenchmarkHandleClientPacketReplayAuthInput(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := player.New(logger, player.MonitoringState{
		IsReplay:    true,
		CurrentTime: time.Unix(0, 0),
	}, nil)
	component.Register(p)
	p.HandleEvents(player.NopEventHandler{})
	p.Ready = true
	p.Alive = true
	p.GameMode = packet.GameTypeSurvival

	// Keep replay deterministic and avoid artificial time skew.
	p.SetTime(time.Unix(0, 0))
	p.Tick()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SetTime(p.Time().Add(50 * time.Millisecond))
		p.Tick()

		auth := packet.Packet(&packet.PlayerAuthInput{
			InputData: protocol.NewBitset(128),
			Tick:      uint64(i + 1),
			Position:  p.Movement().Pos().Add(mgl32.Vec3{0, 1.621, 0}),
			Delta:     p.Movement().Vel(),
			Pitch:     p.Movement().Rotation().X(),
			Yaw:       p.Movement().Rotation().Z(),
			HeadYaw:   p.Movement().Rotation().Y(),
		})
		ctx := context.NewHandlePacketContext(&auth)
		p.HandleClientPacket(ctx)
	}
}

func BenchmarkHandleClientPacketNetworkStackLatency(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := player.New(logger, player.MonitoringState{
		IsReplay:    true,
		CurrentTime: time.Unix(0, 0),
	}, nil)
	component.Register(p)
	p.HandleEvents(player.NopEventHandler{})
	p.Ready = true
	p.Alive = true
	p.GameMode = packet.GameTypeSurvival

	p.ACKs().SetLegacy(true)
	ackRuns := 0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ACKs().Add(&countingACK{count: &ackRuns})
		ts := p.ACKs().Timestamp()
		p.ACKs().Flush()

		raw := packet.Packet(&packet.NetworkStackLatency{Timestamp: ts})
		ctx := context.NewHandlePacketContext(&raw)
		p.HandleClientPacket(ctx)
	}
}
