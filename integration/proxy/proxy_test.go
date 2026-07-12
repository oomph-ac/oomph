package proxy

import (
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestBackendSwapInvalidatesOldGeneration(t *testing.T) {
	old := &fakeBackend{data: minecraft.GameData{EntityRuntimeID: 1}}
	next := &fakeBackend{data: minecraft.GameData{EntityRuntimeID: 9}}
	s := &session{backend: old}
	backend, generation := s.currentBackend()
	if replaced := s.swapBackend(next); replaced != old {
		t.Fatalf("swapBackend() replaced %#v, want old backend", replaced)
	}
	if s.isCurrent(backend, generation) {
		t.Fatal("old backend generation remained current after swap")
	}
	got, gotGeneration := s.currentBackend()
	if got != next || gotGeneration != generation+1 {
		t.Fatalf("current backend = %#v generation %d", got, gotGeneration)
	}
}

func TestRuntimeIDRewritePreservesClientIdentityAcrossBackendSwap(t *testing.T) {
	p := &player.Player{RuntimeId: 27}
	s := &session{player: p, clientRuntimeID: 1}
	clientPacket := &packet.MovePlayer{EntityRuntimeID: 1}
	s.rewriteClientPacket(clientPacket)
	if clientPacket.EntityRuntimeID != 27 {
		t.Fatalf("client runtime ID = %d, want backend ID 27", clientPacket.EntityRuntimeID)
	}
	serverPacket := &packet.MovePlayer{EntityRuntimeID: 27}
	s.rewriteServerPacket(serverPacket)
	if serverPacket.EntityRuntimeID != 1 {
		t.Fatalf("server runtime ID = %d, want stable client ID 1", serverPacket.EntityRuntimeID)
	}
}

func TestTransferResetsClientWorldAcrossBackendDimensions(t *testing.T) {
	data := minecraft.GameData{Dimension: packet.DimensionOverworld}
	packets := transferResetPackets(packet.DimensionOverworld, data)
	var changes []*packet.ChangeDimension
	for _, pk := range packets {
		if change, ok := pk.(*packet.ChangeDimension); ok {
			changes = append(changes, change)
		}
	}
	if len(changes) != 2 {
		t.Fatalf("dimension changes = %d, want fake and destination changes", len(changes))
	}
	if changes[0].Dimension == packet.DimensionOverworld || changes[1].Dimension != packet.DimensionOverworld {
		t.Fatalf("dimension reset sequence = %d -> %d", changes[0].Dimension, changes[1].Dimension)
	}
}

func TestRuntimeIDRewriteCoversSelfActorPackets(t *testing.T) {
	p := &player.Player{RuntimeId: 27}
	s := &session{player: p, clientRuntimeID: 1}
	pk := &packet.ActorEvent{EntityRuntimeID: 27}
	s.rewriteServerPacket(pk)
	if pk.EntityRuntimeID != 1 {
		t.Fatalf("ActorEvent runtime ID = %d, want stable client ID 1", pk.EntityRuntimeID)
	}
}

func TestUniqueIDRewritePreservesClientIdentityAcrossBackendSwap(t *testing.T) {
	p := &player.Player{UniqueId: 84}
	s := &session{player: p, clientUniqueID: 2}
	pk := &packet.UpdateAbilities{AbilityData: protocol.AbilityData{EntityUniqueID: 84}}
	s.rewriteServerPacket(pk)
	if pk.AbilityData.EntityUniqueID != 2 {
		t.Fatalf("ability unique ID = %d, want stable client ID 2", pk.AbilityData.EntityUniqueID)
	}
}

func TestTransferDoesNotReportSuccessWhenStateSyncFails(t *testing.T) {
	want := errors.New("flush failed")
	backend := &fakeBackend{data: minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10}, flushErr: want}
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	component.Register(p)
	p.RuntimeId, p.UniqueId = 9, 10
	s := &session{
		player: p, client: &fakeClient{}, backend: backend,
		clientRuntimeID: 1, clientUniqueID: 2,
	}
	if err := s.resetTransferState(player.BackendTransferState{}); !errors.Is(err, want) {
		t.Fatalf("resetTransferState() error = %v, want %v", err, want)
	}
}

func TestTransferResetSynchronizesWithPlayerTick(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	component.Register(p)
	backend := &fakeBackend{data: minecraft.GameData{EntityRuntimeID: 1}}
	p.SetServerConn(backend)
	go p.StartTicking()
	for i := 0; i < 100; i++ {
		p.TransferServerConn(&fakeBackend{data: minecraft.GameData{EntityRuntimeID: uint64(i + 2)}})
	}
	_ = p.Close()
	select {
	case <-p.CloseChan:
	case <-time.After(time.Second):
		t.Fatal("player did not close")
	}
}

type fakeBackend struct {
	data     minecraft.GameData
	flushErr error
}

func (f *fakeBackend) GameData() minecraft.GameData     { return f.data }
func (*fakeBackend) ReadPacket() (packet.Packet, error) { return nil, nil }
func (*fakeBackend) WritePacket(packet.Packet) error    { return nil }
func (*fakeBackend) DoSpawn() error                     { return nil }
func (f *fakeBackend) Flush() error                     { return f.flushErr }
func (*fakeBackend) Close() error                       { return nil }

type fakeClient struct{}

func (*fakeClient) ReadPacket() (packet.Packet, error) { return nil, errors.New("unused") }
func (*fakeClient) WritePacket(packet.Packet) error    { return nil }
func (*fakeClient) StartGame(minecraft.GameData) error { return nil }
func (*fakeClient) RemoteAddr() net.Addr               { return nil }
func (*fakeClient) Close() error                       { return nil }
