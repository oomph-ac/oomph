package dragonfly

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/oomph-ac/oomph/anticheat/oconfig"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestClientPacketCancelledByOomphDoesNotReachDragonfly(t *testing.T) {
	raw := newBlockingConn()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	component.Register(p)
	conn := newSessionConn(raw, p)
	t.Cleanup(func() { _ = conn.Close() })

	previous := oconfig.Global
	oconfig.Global.CommandName = "ac"
	t.Cleanup(func() { oconfig.Global = previous })
	raw.incoming <- &packet.CommandRequest{CommandLine: "/ac alerts"}
	raw.incoming <- &packet.Text{Message: "forwarded"}

	got, err := conn.ReadPacket()
	if err != nil {
		t.Fatal(err)
	}
	if text, ok := got.(*packet.Text); !ok || text.Message != "forwarded" {
		t.Fatalf("ReadPacket() = %#v, want packet after cancelled command", got)
	}
}

func TestServerPacketIsProcessedBeforeSingleClientWrite(t *testing.T) {
	raw := newBlockingConn()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	component.Register(p)
	conn := newSessionConn(raw, p)
	t.Cleanup(func() { _ = conn.Close() })

	pk := &packet.MobEffect{Tick: 99}
	if err := conn.WritePacket(pk); err != nil {
		t.Fatal(err)
	}
	select {
	case written := <-raw.writes:
		if written != pk {
			t.Fatalf("written packet = %#v, want original pointer %#v", written, pk)
		}
		if pk.Tick != 0 {
			t.Fatalf("MobEffect.Tick = %d, want Oomph-normalized 0", pk.Tick)
		}
	default:
		t.Fatal("processed packet was not written")
	}
	select {
	case duplicate := <-raw.writes:
		t.Fatalf("packet written more than once: %#v", duplicate)
	default:
	}
}

func TestStartGameConnectsOomphOutputToDragonfly(t *testing.T) {
	raw := newBlockingConn()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	component.Register(p)
	conn := newSessionConn(raw, p)
	t.Cleanup(func() { _ = conn.Close() })
	data := minecraft.GameData{
		EntityRuntimeID: 1,
		EntityUniqueID:  42,
		PlayerGameMode:  packet.GameTypeSurvival,
	}
	if err := conn.StartGameContext(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	if p.ServerConn() == nil {
		t.Fatal("StartGameContext did not attach the embedded server output")
	}
	if p.RuntimeId != 1 || p.UniqueId != 42 {
		t.Fatalf("player identity = runtime %d unique %d", p.RuntimeId, p.UniqueId)
	}

	want := &packet.Text{Message: "generated"}
	if err := p.SendPacketToServer(want); err != nil {
		t.Fatal(err)
	}
	got, err := conn.ReadPacket()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("ReadPacket() = %#v, want generated packet %#v", got, want)
	}
}

func TestInjectedPacketsUseFIFOOrder(t *testing.T) {
	raw := newBlockingConn()
	conn := newSessionConn(raw, nil)
	t.Cleanup(func() { _ = conn.Close() })
	sink := embeddedServerConn{conn: conn}

	if err := sink.WritePacket(&packet.Text{Message: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := sink.WritePacket(&packet.Text{Message: "second"}); err != nil {
		t.Fatal(err)
	}

	first, err := conn.ReadPacket()
	if err != nil || first.(*packet.Text).Message != "first" {
		t.Fatalf("first read = %#v, %v", first, err)
	}
	second, err := conn.ReadPacket()
	if err != nil || second.(*packet.Text).Message != "second" {
		t.Fatalf("second read = %#v, %v", second, err)
	}
}

func TestInjectedPacketWakesBlockedRead(t *testing.T) {
	raw := newBlockingConn()
	conn := newSessionConn(raw, nil)
	t.Cleanup(func() { _ = conn.Close() })
	result := make(chan packet.Packet, 1)
	go func() {
		pk, _ := conn.ReadPacket()
		result <- pk
	}()

	if err := (&embeddedServerConn{conn: conn}).WritePacket(&packet.Text{Message: "wake"}); err != nil {
		t.Fatal(err)
	}
	select {
	case pk := <-result:
		if pk.(*packet.Text).Message != "wake" {
			t.Fatalf("packet = %#v", pk)
		}
	case <-time.After(time.Second):
		t.Fatal("injected packet did not wake blocked read")
	}
}

func TestInjectedPacketsSupportConcurrentProducerAndConsumer(t *testing.T) {
	raw := newBlockingConn()
	conn := newSessionConn(raw, nil)
	t.Cleanup(func() { _ = conn.Close() })
	sink := embeddedServerConn{conn: conn}
	const count = 1_000
	errCh := make(chan error, 2)
	go func() {
		for i := 0; i < count; i++ {
			if err := sink.WritePacket(&packet.Text{Message: strconv.Itoa(i)}); err != nil {
				errCh <- err
				return
			}
		}
		errCh <- nil
	}()
	go func() {
		for i := 0; i < count; i++ {
			pk, err := conn.ReadPacket()
			if err != nil {
				errCh <- err
				return
			}
			if got := pk.(*packet.Text).Message; got != strconv.Itoa(i) {
				errCh <- fmt.Errorf("packet %d = %q", i, got)
				return
			}
		}
		errCh <- nil
	}()
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

type blockingConn struct {
	closed   chan struct{}
	incoming chan packet.Packet
	writes   chan packet.Packet
	closeOne sync.Once
}

func newBlockingConn() *blockingConn {
	return &blockingConn{
		closed:   make(chan struct{}),
		incoming: make(chan packet.Packet, 8),
		writes:   make(chan packet.Packet, 8),
	}
}

func (c *blockingConn) ReadPacket() (packet.Packet, error) {
	select {
	case pk := <-c.incoming:
		return pk, nil
	case <-c.closed:
		return nil, io.EOF
	}
}
func (c *blockingConn) WritePacket(pk packet.Packet) error {
	select {
	case c.writes <- pk:
		return nil
	case <-c.closed:
		return io.EOF
	}
}
func (c *blockingConn) Close() error {
	c.closeOne.Do(func() { close(c.closed) })
	return nil
}
func (c *blockingConn) IdentityData() login.IdentityData { return login.IdentityData{} }
func (c *blockingConn) ClientData() login.ClientData     { return login.ClientData{} }
func (c *blockingConn) ClientCacheEnabled() bool         { return false }
func (c *blockingConn) ChunkRadius() int                 { return 0 }
func (c *blockingConn) Latency() time.Duration           { return 0 }
func (c *blockingConn) Flush() error                     { return nil }
func (c *blockingConn) RemoteAddr() net.Addr             { return nil }
func (c *blockingConn) StartGameContext(context.Context, minecraft.GameData) error {
	return nil
}
