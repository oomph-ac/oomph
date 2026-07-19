package proxy

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component"
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

func (b adapterBackend) GameData() minecraft.GameData     { return b.data }
func (adapterBackend) ReadPacket() (packet.Packet, error) { return nil, nil }
func (adapterBackend) WritePacket(packet.Packet) error    { return nil }
func (adapterBackend) DoSpawn() error                     { return nil }
func (adapterBackend) Flush() error                       { return nil }
func (adapterBackend) Close() error                       { return nil }
