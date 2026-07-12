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
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestListenerCompressionDefaultsToSnappy(t *testing.T) {
	got := listenerCompression(nil)
	if got.EncodeCompression() != packet.SnappyCompression.EncodeCompression() {
		t.Fatalf("default compression ID = %d, want Snappy ID %d", got.EncodeCompression(), packet.SnappyCompression.EncodeCompression())
	}
}

func TestListenerCompressionPreservesExplicitSetting(t *testing.T) {
	got := listenerCompression(packet.FlateCompression)
	if got.EncodeCompression() != packet.FlateCompression.EncodeCompression() {
		t.Fatalf("compression ID = %d, want explicit Flate ID %d", got.EncodeCompression(), packet.FlateCompression.EncodeCompression())
	}
}

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
	c := newSessionConn(newBlockingConn(), p)
	l := &listener{}
	if err := l.Disconnect(c, "rejected"); err != nil {
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

func TestListenerClosesWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l, err := Listener(ctx, Config{Address: "127.0.0.1:0"})(server.Config{
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		StatusProvider: minecraft.NewStatusProvider("Oomph test", "Oomph test"),
	})
	if err != nil {
		t.Fatalf("Listener() error = %v", err)
	}
	acceptErr := make(chan error, 1)
	go func() {
		_, err := l.Accept()
		acceptErr <- err
	}()
	cancel()
	select {
	case err := <-acceptErr:
		if err == nil {
			t.Fatal("Accept returned nil after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not close the listener")
	}
}
