package proxy

import (
	"log/slog"
	"testing"
	"time"

	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestOomphHandlerTransferSynchronizesWithPlayerTick(t *testing.T) {
	pl := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	component.Register(pl)
	initial := adapterBackend{data: minecraft.GameData{EntityRuntimeID: 1}}
	pl.SetServerConn(initial)
	go pl.StartTicking()
	h := &oomphHandler{player: pl}
	for i := 0; i < 100; i++ {
		if err := h.TransferBackend(adapterBackend{data: minecraft.GameData{EntityRuntimeID: uint64(i + 2)}}); err != nil {
			t.Fatal(err)
		}
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

func (b adapterBackend) GameData() minecraft.GameData     { return b.data }
func (adapterBackend) ReadPacket() (packet.Packet, error) { return nil, nil }
func (adapterBackend) WritePacket(packet.Packet) error    { return nil }
func (adapterBackend) DoSpawn() error                     { return nil }
func (adapterBackend) Flush() error                       { return nil }
func (adapterBackend) Close() error                       { return nil }
