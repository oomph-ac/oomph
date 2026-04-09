package integration_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func newReplayPlayer(tb testing.TB) *player.Player {
	tb.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := player.New(logger, player.MonitoringState{
		IsReplay:    true,
		CurrentTime: time.Unix(0, 0),
	}, nil)
	p.HandleEvents(player.NopEventHandler{})
	return p
}

type mockDetection struct {
	typ      string
	subType  string
	punish   bool
	metadata *player.DetectionMetadata
}

func newMockDetection(typ, sub string, failBuffer, maxBuffer, maxVl float64) *mockDetection {
	return &mockDetection{
		typ:     typ,
		subType: sub,
		punish:  false,
		metadata: &player.DetectionMetadata{
			FailBuffer:    failBuffer,
			MaxBuffer:     maxBuffer,
			MaxViolations: maxVl,
		},
	}
}

func (d *mockDetection) Type() string                           { return d.typ }
func (d *mockDetection) SubType() string                        { return d.subType }
func (d *mockDetection) Description() string                    { return "test detection" }
func (d *mockDetection) Punishable() bool                       { return d.punish }
func (d *mockDetection) Metadata() *player.DetectionMetadata    { return d.metadata }
func (d *mockDetection) Detect(packet.Packet)                   {}
