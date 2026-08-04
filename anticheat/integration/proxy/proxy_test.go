package proxy

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component"
	playercontext "github.com/oomph-ac/oomph/anticheat/player/context"
	proxycore "github.com/oomph-ac/oomph/transferproxy"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestOomphHandlerTransferSynchronizesWithPlayerTick(t *testing.T) {
	pl := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now(), IsReplay: true}, nil)
	component.Register(pl)
	initial := adapterBackend{data: minecraft.GameData{EntityRuntimeID: 1}}
	pl.SetServerConn(initial)
	h := &oomphHandler{player: pl}
	entered, release := make(chan struct{}), make(chan struct{})
	transferDone := make(chan error, 1)
	go func() {
		transferDone <- h.TransferBackend(&blockingGameDataBackend{
			adapterBackend: adapterBackend{data: minecraft.GameData{EntityRuntimeID: 2}},
			entered:        entered,
			release:        release,
		})
	}()
	<-entered

	tickStarted, tickDone := make(chan struct{}), make(chan bool, 1)
	go func() {
		close(tickStarted)
		tickDone <- pl.Tick()
	}()
	<-tickStarted
	select {
	case <-tickDone:
		t.Fatal("player tick completed while backend transfer held the processing lock")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-transferDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-tickDone:
	case <-time.After(time.Second):
		t.Fatal("player tick did not resume after backend transfer")
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pl.CloseChan:
	case <-time.After(time.Second):
		t.Fatal("player did not close")
	}
}

func TestAdaptPacketContextsPropagatesHandlingResults(t *testing.T) {
	replacement := &packet.Text{Message: "replacement"}
	untouched := &packet.Text{Message: "untouched"}
	batch := []*PacketContext{
		proxycore.NewPacketContext(&packet.Text{Message: "cancel me"}),
		proxycore.NewPacketContext(&packet.Text{Message: "replace me"}),
		proxycore.NewPacketContext(untouched),
	}
	adaptPacketContexts(batch, func(converted []*playercontext.HandlePacketContext) {
		if len(converted) != len(batch) {
			t.Fatalf("converted batch size = %d, want %d", len(converted), len(batch))
		}
		converted[0].Cancel()
		*converted[1].Packet() = replacement
		converted[1].SetModified()
	})
	if !batch[0].Cancelled() {
		t.Fatal("cancellation was not propagated to the proxy context")
	}
	if batch[1].Packet() != packet.Packet(replacement) || !batch[1].Modified() {
		t.Fatalf("proxy context packet = %#v modified %t, want the replacement packet marked modified", batch[1].Packet(), batch[1].Modified())
	}
	if batch[2].Cancelled() || batch[2].Modified() {
		t.Fatal("untouched packet was cancelled or marked modified")
	}
	if batch[2].Packet() != packet.Packet(untouched) {
		t.Fatalf("untouched context packet = %#v, want the original packet", batch[2].Packet())
	}
}

type adapterBackend struct {
	data minecraft.GameData
}

type blockingGameDataBackend struct {
	adapterBackend
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingGameDataBackend) GameData() minecraft.GameData {
	b.once.Do(func() {
		close(b.entered)
		<-b.release
	})
	return b.adapterBackend.GameData()
}

func (b adapterBackend) GameData() minecraft.GameData      { return b.data }
func (adapterBackend) ReadPacket() (packet.Packet, error)  { return nil, nil }
func (adapterBackend) ReadBatch() ([]packet.Packet, error) { return nil, nil }
func (adapterBackend) WritePacket(packet.Packet) error     { return nil }
func (adapterBackend) DoSpawn() error                      { return nil }
func (adapterBackend) Flush() error                        { return nil }
func (adapterBackend) Close() error                        { return nil }
