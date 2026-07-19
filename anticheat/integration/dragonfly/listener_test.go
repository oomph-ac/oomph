package dragonfly

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/oomph-ac/oomph/anticheat/player"
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

func TestWrapUsesSuppliedListenerFactory(t *testing.T) {
	base := newStubListener()
	var gotConfig server.Config
	factory := Wrap(context.Background(), func(conf server.Config) (server.Listener, error) {
		gotConfig = conf
		return base, nil
	}, nil)
	wantConfig := server.Config{Log: slog.Default(), Name: "wrapped"}

	l, err := factory(wantConfig)
	if err != nil {
		t.Fatalf("Wrap() factory error = %v", err)
	}
	if gotConfig.Name != wantConfig.Name {
		t.Fatalf("factory config name = %q, want %q", gotConfig.Name, wantConfig.Name)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case <-base.closed:
	default:
		t.Fatal("Close did not delegate to the supplied listener")
	}
}

func TestWrapRejectsNilListener(t *testing.T) {
	_, err := Wrap(context.Background(), func(server.Config) (server.Listener, error) {
		return nil, nil
	}, nil)(server.Config{Log: slog.Default()})
	if err == nil {
		t.Fatal("Wrap() accepted a nil listener")
	}
}

func TestWrappedListenerDisconnectDelegatesAndClosesOomphPlayer(t *testing.T) {
	base := newStubListener()
	l, err := Wrap(context.Background(), func(server.Config) (server.Listener, error) {
		return base, nil
	}, nil)(server.Config{Log: slog.Default()})
	if err != nil {
		t.Fatalf("Wrap() factory error = %v", err)
	}
	raw := newBlockingConn()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	c := newSessionConn(raw, p)

	if err := l.Disconnect(c, "rejected"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if base.disconnected != raw || base.reason != "rejected" {
		t.Fatalf("delegated Disconnect() = (%T, %q), want (%T, %q)", base.disconnected, base.reason, raw, "rejected")
	}
	select {
	case <-p.CloseChan:
	case <-time.After(time.Second):
		t.Fatal("Disconnect did not close the Oomph player")
	}
}

func TestWrappedListenerRejectsAndClosesUnexpectedConnectionType(t *testing.T) {
	raw := newBlockingConn()
	base := newStubListener()
	base.accept = raw
	l, err := Wrap(context.Background(), func(server.Config) (server.Listener, error) {
		return base, nil
	}, nil)(server.Config{Log: slog.Default()})
	if err != nil {
		t.Fatalf("Wrap() factory error = %v", err)
	}

	if _, err := l.Accept(); err == nil {
		t.Fatal("Accept() accepted a non-minecraft connection")
	}
	select {
	case <-raw.closed:
	default:
		t.Fatal("Accept did not close the rejected connection")
	}
}

func TestWrappedListenerClosesWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	base := newStubListener()
	_, err := Wrap(ctx, func(server.Config) (server.Listener, error) {
		return base, nil
	}, nil)(server.Config{Log: slog.Default()})
	if err != nil {
		t.Fatalf("Wrap() factory error = %v", err)
	}

	cancel()
	select {
	case <-base.closed:
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not close the supplied listener")
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

type stubListener struct {
	accept       session.Conn
	acceptErr    error
	disconnected session.Conn
	reason       string
	closed       chan struct{}
	closeOnce    sync.Once
}

func newStubListener() *stubListener {
	return &stubListener{closed: make(chan struct{})}
}

func (l *stubListener) Accept() (session.Conn, error) {
	return l.accept, l.acceptErr
}

func (l *stubListener) Disconnect(conn session.Conn, reason string) error {
	l.disconnected, l.reason = conn, reason
	return nil
}

func (l *stubListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}
