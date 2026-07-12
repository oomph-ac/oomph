package dragonfly

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
)

func TestListenerFactoryListensOnEphemeralAddress(t *testing.T) {
	factory := Listener(context.Background(), Config{Address: "127.0.0.1:0"})
	l, err := factory(server.Config{
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		Name:           "Oomph test",
		StatusProvider: minecraft.NewStatusProvider("Oomph test", "Oomph test"),
	})
	if err != nil {
		t.Fatalf("Listener() error = %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDisconnectClosesOomphPlayerLifecycle(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	l := &listener{}
	if err := l.Disconnect(p, "rejected"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	select {
	case <-p.CloseChan:
	case <-time.After(time.Second):
		t.Fatal("Disconnect did not close the Oomph player")
	}
}

func TestListenerFactoryRequiresAddress(t *testing.T) {
	_, err := Listener(context.Background(), Config{})(server.Config{Log: slog.Default()})
	if err == nil {
		t.Fatal("Listener() accepted an empty address")
	}
}
