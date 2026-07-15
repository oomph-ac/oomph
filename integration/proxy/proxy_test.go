package proxy

import (
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestBatchForwardingDefaultsDisabled(t *testing.T) {
	if (Config{}).EnableBatchForwarding {
		t.Fatal("batch forwarding must be opt-in")
	}
}

func TestBatchForwardingCapabilitiesRejectLegacyBackend(t *testing.T) {
	_, err := batchCapabilities("backend", &fakeBackend{})
	if err == nil || !strings.Contains(err.Error(), "batch forwarding requires") {
		t.Fatalf("batchCapabilities() error = %v, want descriptive capability error", err)
	}
}

func TestClientBatchForwardsPacketsInOrderImmediately(t *testing.T) {
	backend := newFakeBatchBackend()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	s := &session{player: p, backend: backend}
	first := &packet.Text{Message: "first"}
	second := &packet.Text{Message: "second"}

	if err := s.forwardClientBatch([]packet.Packet{first, second}); err != nil {
		t.Fatalf("forwardClientBatch: %v", err)
	}
	if len(backend.immediate) != 1 {
		t.Fatalf("immediate writes = %d, want 1", len(backend.immediate))
	}
	got := backend.immediate[0]
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("immediate batch = %#v, want original packet order", got)
	}
}

func TestClientBatchFlushesWhenNoPacketsSurvive(t *testing.T) {
	backend := newFakeBatchBackend()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	s := &session{player: p, backend: backend}

	if err := s.forwardClientBatch(nil); err != nil {
		t.Fatalf("forwardClientBatch: %v", err)
	}
	if len(backend.immediate) != 1 || len(backend.immediate[0]) != 0 {
		t.Fatalf("immediate writes = %#v, want one empty flush", backend.immediate)
	}
}

func TestBackendBatchForwardsPacketsInOrderImmediately(t *testing.T) {
	client := newFakeBatchClient()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	s := &session{player: p, client: client, state: newBackendStateTracker()}
	first := &packet.Text{Message: "first"}
	second := &packet.Text{Message: "second"}

	transfer, err := s.forwardBackendBatch([]packet.Packet{first, second})
	if err != nil {
		t.Fatalf("forwardBackendBatch: %v", err)
	}
	if transfer != nil {
		t.Fatalf("forwardBackendBatch transfer = %#v, want nil", transfer)
	}
	if len(client.immediate) != 1 {
		t.Fatalf("immediate writes = %d, want 1", len(client.immediate))
	}
	got := client.immediate[0]
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("immediate batch = %#v, want original packet order", got)
	}
}

func TestBackendBatchFlushesPrefixAndDropsTailAtTransfer(t *testing.T) {
	client := newFakeBatchClient()
	p := player.New(slog.Default(), player.MonitoringState{CurrentTime: time.Now()}, nil)
	s := &session{player: p, client: client, state: newBackendStateTracker()}
	first := &packet.Text{Message: "first"}
	handoff := &packet.Transfer{Address: "127.0.0.1", Port: 19133}
	tail := &packet.Text{Message: "old backend tail"}

	transfer, err := s.forwardBackendBatch([]packet.Packet{first, handoff, tail})
	if err != nil {
		t.Fatalf("forwardBackendBatch: %v", err)
	}
	if transfer != handoff {
		t.Fatalf("forwardBackendBatch transfer = %#v, want %#v", transfer, handoff)
	}
	if len(client.immediate) != 1 || len(client.immediate[0]) != 1 || client.immediate[0][0] != first {
		t.Fatalf("immediate writes = %#v, want only pre-transfer packet", client.immediate)
	}
}

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
	data := minecraft.GameData{Dimension: packet.DimensionOverworld, Difficulty: 3, Pitch: 12, Yaw: 34}
	packets := transferResetPackets(packet.DimensionOverworld, data)
	var changes []*packet.ChangeDimension
	var stoppedSounds, stoppedRain, stoppedThunder, setDifficulty, resetRotation bool
	for _, pk := range packets {
		switch pk := pk.(type) {
		case *packet.ChangeDimension:
			change := pk
			changes = append(changes, change)
		case *packet.StopSound:
			stoppedSounds = pk.StopAll
		case *packet.LevelEvent:
			stoppedRain = stoppedRain || pk.EventType == packet.LevelEventStopRaining && pk.EventData == 10_000
			stoppedThunder = stoppedThunder || pk.EventType == packet.LevelEventStopThunderstorm
		case *packet.SetDifficulty:
			setDifficulty = pk.Difficulty == uint32(data.Difficulty)
		case *packet.MovePlayer:
			resetRotation = pk.Pitch == data.Pitch && pk.Yaw == data.Yaw
		}
	}
	if len(changes) != 2 {
		t.Fatalf("dimension changes = %d, want fake and destination changes", len(changes))
	}
	if changes[0].Dimension == packet.DimensionOverworld || changes[1].Dimension != packet.DimensionOverworld {
		t.Fatalf("dimension reset sequence = %d -> %d", changes[0].Dimension, changes[1].Dimension)
	}
	if !stoppedSounds || !stoppedRain || !stoppedThunder || !setDifficulty || !resetRotation {
		t.Fatalf("destination reset missing: sounds=%t rain=%t thunder=%t difficulty=%t rotation=%t", stoppedSounds, stoppedRain, stoppedThunder, setDifficulty, resetRotation)
	}
}

func TestBackendStateTrackerClearsSpectrumTransferState(t *testing.T) {
	entryID := uuid.New()
	tracker := newBackendStateTracker()
	tracker.handle(&packet.AddActor{EntityUniqueID: 11})
	tracker.handle(&packet.AddItemActor{EntityUniqueID: 12})
	tracker.handle(&packet.AddPainting{EntityUniqueID: 13})
	tracker.handle(&packet.BossEvent{BossEntityUniqueID: 14, EventType: packet.BossEventShow})
	tracker.handle(&packet.PlayerList{ActionType: packet.PlayerListActionAdd, Entries: []protocol.PlayerListEntry{{UUID: entryID}}})
	tracker.handle(&packet.SetDisplayObjective{ObjectiveName: "kills"})

	packets := tracker.clearPackets()
	var entities, bossBars, players, objectives int
	for _, pk := range packets {
		switch pk := pk.(type) {
		case *packet.RemoveActor:
			if pk.EntityUniqueID >= 11 && pk.EntityUniqueID <= 13 {
				entities++
			}
		case *packet.BossEvent:
			if pk.BossEntityUniqueID == 14 && pk.EventType == packet.BossEventHide {
				bossBars++
			}
		case *packet.PlayerList:
			if pk.ActionType == packet.PlayerListActionRemove && len(pk.Entries) == 1 && pk.Entries[0].UUID == entryID {
				players++
			}
		case *packet.RemoveObjective:
			if pk.ObjectiveName == "kills" {
				objectives++
			}
		}
	}
	if entities != 3 || bossBars != 1 || players != 1 || objectives != 1 {
		t.Fatalf("clear packets: entities=%d bossBars=%d players=%d objectives=%d", entities, bossBars, players, objectives)
	}
	if packets := tracker.clearPackets(); len(packets) != 0 {
		t.Fatalf("second clear emitted %d stale packets", len(packets))
	}
}

func TestBackendStateTrackerHonoursRemovalPackets(t *testing.T) {
	entryID := uuid.New()
	tracker := newBackendStateTracker()
	tracker.handle(&packet.AddActor{EntityUniqueID: 11})
	tracker.handle(&packet.RemoveActor{EntityUniqueID: 11})
	tracker.handle(&packet.PlayerList{ActionType: packet.PlayerListActionAdd, Entries: []protocol.PlayerListEntry{{UUID: entryID}}})
	tracker.handle(&packet.PlayerList{ActionType: packet.PlayerListActionRemove, Entries: []protocol.PlayerListEntry{{UUID: entryID}}})
	tracker.handle(&packet.SetDisplayObjective{ObjectiveName: "kills"})
	tracker.handle(&packet.RemoveObjective{ObjectiveName: "kills"})
	if packets := tracker.clearPackets(); len(packets) != 0 {
		t.Fatalf("clear emitted %d packets for removed state", len(packets))
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

func TestRuntimeIDRewriteAvoidsBackendEntityCollision(t *testing.T) {
	p := &player.Player{RuntimeId: 27, UniqueId: 84}
	s := &session{player: p, clientRuntimeID: 1, clientUniqueID: 2}
	pk := &packet.MoveActorAbsolute{EntityRuntimeID: 1}
	if !s.rewriteServerPacket(pk) {
		t.Fatal("collision packet was suppressed")
	}
	if pk.EntityRuntimeID != math.MaxInt64 {
		t.Fatalf("collision runtime ID = %d, want sentinel %d", pk.EntityRuntimeID, int64(math.MaxInt64))
	}
	clientPacket := &packet.Interact{TargetEntityRuntimeID: math.MaxInt64}
	s.rewriteClientPacket(clientPacket)
	if clientPacket.TargetEntityRuntimeID != 1 {
		t.Fatalf("collision target runtime ID = %d, want backend entity ID 1", clientPacket.TargetEntityRuntimeID)
	}
	painting := &packet.AddPainting{EntityRuntimeID: 1, EntityUniqueID: 2}
	s.rewriteServerPacket(painting)
	if painting.EntityRuntimeID != math.MaxInt64 || painting.EntityUniqueID != math.MaxInt64 {
		t.Fatalf("painting collision IDs = %d/%d, want sentinel", painting.EntityRuntimeID, painting.EntityUniqueID)
	}
}

func TestRuntimeIDRewriteSuppressesBackendSelfSpawn(t *testing.T) {
	p := &player.Player{RuntimeId: 27}
	s := &session{player: p, clientRuntimeID: 1}
	if s.rewriteServerPacket(&packet.AddActor{EntityRuntimeID: 27}) {
		t.Fatal("backend self AddActor was forwarded")
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
		state: newBackendStateTracker(),
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
		_, _ = p.TransferServerConn(&fakeBackend{data: minecraft.GameData{EntityRuntimeID: uint64(i + 2)}})
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

type fakeBatchBackend struct {
	*fakeBackend
	batches   [][]packet.Packet
	immediate [][]packet.Packet
}

func newFakeBatchBackend() *fakeBatchBackend {
	return &fakeBatchBackend{fakeBackend: &fakeBackend{}}
}

func (f *fakeBatchBackend) ReadBatch() ([]packet.Packet, error) {
	if len(f.batches) == 0 {
		return nil, io.EOF
	}
	batch := f.batches[0]
	f.batches = f.batches[1:]
	return batch, nil
}

func (f *fakeBatchBackend) WritePacketImmediate(packets ...packet.Packet) error {
	f.immediate = append(f.immediate, append([]packet.Packet(nil), packets...))
	return nil
}

type fakeClient struct {
	packets []packet.Packet
}

func (*fakeClient) ReadPacket() (packet.Packet, error) { return nil, errors.New("unused") }
func (f *fakeClient) WritePacket(pk packet.Packet) error {
	f.packets = append(f.packets, pk)
	return nil
}
func (*fakeClient) StartGame(minecraft.GameData) error { return nil }
func (*fakeClient) RemoteAddr() net.Addr               { return nil }
func (*fakeClient) Close() error                       { return nil }

type fakeBatchClient struct {
	*fakeClient
	batches   [][]packet.Packet
	immediate [][]packet.Packet
}

func newFakeBatchClient() *fakeBatchClient {
	return &fakeBatchClient{fakeClient: &fakeClient{}}
}

func (f *fakeBatchClient) ReadBatch() ([]packet.Packet, error) {
	if len(f.batches) == 0 {
		return nil, io.EOF
	}
	batch := f.batches[0]
	f.batches = f.batches[1:]
	return batch, nil
}

func (f *fakeBatchClient) WritePacketImmediate(packets ...packet.Packet) error {
	f.immediate = append(f.immediate, append([]packet.Packet(nil), packets...))
	return nil
}
