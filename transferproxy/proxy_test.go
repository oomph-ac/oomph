package proxy

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestDefaultDialPreservesXBLIdentityData(t *testing.T) {
	listener, err := (minecraft.ListenConfig{AuthenticationDisabled: true}).Listen("raknet", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	accepted := make(chan *minecraft.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		backend := conn.(*minecraft.Conn)
		accepted <- backend
		if err := backend.StartGame(minecraft.GameData{}); err != nil {
			acceptErr <- err
		}
	}()

	want := login.IdentityData{
		DisplayName: "ProxyPlayer",
		Identity:    uuid.NewString(),
		XUID:        "2533274790395904",
	}
	backend, err := defaultDial(5*time.Second, false)(context.Background(), listener.Addr().String(), want, login.ClientData{}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backend.Close() }()

	select {
	case err := <-acceptErr:
		t.Fatal(err)
	case conn := <-accepted:
		defer func() { _ = conn.Close() }()
		got := conn.IdentityData()
		if got.XUID != want.XUID {
			t.Fatalf("backend XUID = %q, want %q", got.XUID, want.XUID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for backend login")
	}
}

func TestDefaultDialHonoursContextCancellation(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = defaultDial(time.Second, false)(ctx, conn.LocalAddr().String(), login.IdentityData{}, login.ClientData{}, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("defaultDial() error = %v, want context.Canceled", err)
	}
}

func TestDefaultDialAutomaticallyFlushesPackets(t *testing.T) {
	listener, err := (minecraft.ListenConfig{AuthenticationDisabled: true}).Listen("raknet", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	accepted := make(chan *minecraft.Conn, 1)
	startErr := make(chan error, 1)
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			startErr <- err
			return
		}
		server := raw.(*minecraft.Conn)
		accepted <- server
		startErr <- server.StartGame(minecraft.GameData{EntityRuntimeID: 1, EntityUniqueID: 2})
	}()

	backend, err := defaultDial(5*time.Second, false)(context.Background(), listener.Addr().String(), login.IdentityData{}, login.ClientData{}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backend.Close() }()
	server := <-accepted
	defer func() { _ = server.Close() }()
	if err := backend.DoSpawn(); err != nil {
		t.Fatal(err)
	}
	if err := <-startErr; err != nil {
		t.Fatal(err)
	}
	if err := server.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := backend.WritePacket(&packet.Text{Message: "automatically flushed"}); err != nil {
		t.Fatal(err)
	}
	pk, err := server.ReadPacket()
	if err != nil {
		t.Fatalf("server did not receive packet without an explicit flush: %v", err)
	}
	text, ok := pk.(*packet.Text)
	if !ok || text.Message != "automatically flushed" {
		t.Fatalf("server received %#v, want the forwarded text packet", pk)
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
	s := &session{backendRuntimeID: 27, clientRuntimeID: 1}
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
	tracker.handle(&packet.AddActor{EntityUniqueID: 11}, 27)
	tracker.handle(&packet.AddItemActor{EntityUniqueID: 12}, 27)
	tracker.handle(&packet.AddPainting{EntityUniqueID: 13}, 27)
	tracker.handle(&packet.BossEvent{BossEntityUniqueID: 14, EventType: packet.BossEventShow}, 27)
	tracker.handle(&packet.MobEffect{EntityRuntimeID: 27, EffectType: 15, Operation: packet.MobEffectModify}, 27)
	tracker.handle(&packet.MobEffect{EntityRuntimeID: 99, EffectType: 15, Operation: packet.MobEffectRemove}, 27)
	tracker.handle(&packet.PlayerList{Entries: []protocol.PlayerListEntry{{ActionType: protocol.PlayerListActionAdd, UUID: entryID}}}, 27)
	tracker.handle(&packet.SetDisplayObjective{ObjectiveName: "kills"}, 27)

	packets := tracker.clearPackets(27)
	var entities, bossBars, effects, players, objectives int
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
		case *packet.MobEffect:
			if pk.EntityRuntimeID == 27 && pk.EffectType == 15 && pk.Operation == packet.MobEffectRemove {
				effects++
			}
		case *packet.PlayerList:
			if len(pk.Entries) == 1 && pk.Entries[0].ActionType == protocol.PlayerListActionRemove && pk.Entries[0].UUID == entryID {
				players++
			}
		case *packet.RemoveObjective:
			if pk.ObjectiveName == "kills" {
				objectives++
			}
		}
	}
	if entities != 3 || bossBars != 1 || effects != 1 || players != 1 || objectives != 1 {
		t.Fatalf("clear packets: entities=%d bossBars=%d effects=%d players=%d objectives=%d", entities, bossBars, effects, players, objectives)
	}
	if packets := tracker.clearPackets(27); len(packets) != 0 {
		t.Fatalf("second clear emitted %d stale packets", len(packets))
	}
}

func TestBackendStateTrackerHonoursRemovalPackets(t *testing.T) {
	entryID := uuid.New()
	tracker := newBackendStateTracker()
	tracker.handle(&packet.AddActor{EntityUniqueID: 11}, 1)
	tracker.handle(&packet.RemoveActor{EntityUniqueID: 11}, 1)
	tracker.handle(&packet.PlayerList{Entries: []protocol.PlayerListEntry{{ActionType: protocol.PlayerListActionAdd, UUID: entryID}}}, 1)
	tracker.handle(&packet.PlayerList{Entries: []protocol.PlayerListEntry{{ActionType: protocol.PlayerListActionRemove, UUID: entryID}}}, 1)
	tracker.handle(&packet.SetDisplayObjective{ObjectiveName: "kills"}, 1)
	tracker.handle(&packet.RemoveObjective{ObjectiveName: "kills"}, 1)
	if packets := tracker.clearPackets(1); len(packets) != 0 {
		t.Fatalf("clear emitted %d packets for removed state", len(packets))
	}
}

func TestRuntimeIDRewriteCoversSelfActorPackets(t *testing.T) {
	s := &session{backendRuntimeID: 27, clientRuntimeID: 1}
	pk := &packet.ActorEvent{EntityRuntimeID: 27}
	s.rewriteServerPacket(pk)
	if pk.EntityRuntimeID != 1 {
		t.Fatalf("ActorEvent runtime ID = %d, want stable client ID 1", pk.EntityRuntimeID)
	}
}

func TestRuntimeIDRewriteAvoidsBackendEntityCollision(t *testing.T) {
	s := &session{backendRuntimeID: 27, backendUniqueID: 84, clientRuntimeID: 1, clientUniqueID: 2}
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
	s := &session{backendRuntimeID: 27, clientRuntimeID: 1}
	if s.rewriteServerPacket(&packet.AddActor{EntityRuntimeID: 27}) {
		t.Fatal("backend self AddActor was forwarded")
	}
}

func TestUniqueIDRewritePreservesClientIdentityAcrossBackendSwap(t *testing.T) {
	s := &session{backendUniqueID: 84, clientUniqueID: 2}
	pk := &packet.UpdateAbilities{AbilityData: protocol.AbilityData{EntityUniqueID: 84}}
	s.rewriteServerPacket(pk)
	if pk.AbilityData.EntityUniqueID != 2 {
		t.Fatalf("ability unique ID = %d, want stable client ID 2", pk.AbilityData.EntityUniqueID)
	}
}

func TestTransferDoesNotReportSuccessWhenStateSyncFails(t *testing.T) {
	want := errors.New("flush failed")
	backend := &fakeBackend{data: minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10}, flushErr: want}
	s := &session{
		handler: NopHandler{}, client: &fakeClient{}, backend: backend,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}
	if err := s.resetTransferState(); !errors.Is(err, want) {
		t.Fatalf("resetTransferState() error = %v, want %v", err, want)
	}
}

func TestTransferUsesHandlerChunkRadius(t *testing.T) {
	var got int32
	backend := &fakeBackend{
		data: minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		write: func(pk packet.Packet) error {
			if request, ok := pk.(*packet.RequestChunkRadius); ok {
				got = request.ChunkRadius
			}
			return nil
		},
	}
	s := &session{
		handler: radiusHandler{radius: 12}, client: &fakeClient{}, backend: backend,
		clientRuntimeID: 1, clientUniqueID: 2, chunkRadius: 8,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}
	if err := s.resetTransferState(); err != nil {
		t.Fatal(err)
	}
	if got != 12 {
		t.Fatalf("transfer chunk radius = %d, want handler radius 12", got)
	}
}

func TestTransferClampsChunkRadius(t *testing.T) {
	for _, test := range []struct {
		name   string
		radius int32
		want   int32
	}{
		{name: "negative", radius: -4, want: 1},
		{name: "above protocol maximum", radius: 300, want: 255},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got *packet.RequestChunkRadius
			backend := &fakeBackend{
				data: minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
				write: func(pk packet.Packet) error {
					if request, ok := pk.(*packet.RequestChunkRadius); ok {
						got = request
					}
					return nil
				},
			}
			s := &session{
				handler: radiusHandler{radius: test.radius}, client: &fakeClient{}, backend: backend,
				clientRuntimeID: 1, clientUniqueID: 2,
				backendRuntimeID: 9, backendUniqueID: 10,
				state: newBackendStateTracker(),
			}
			if err := s.resetTransferState(); err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("backend did not receive RequestChunkRadius")
			}
			if got.ChunkRadius != test.want || got.MaxChunkRadius != uint8(test.want) {
				t.Fatalf("chunk radius = %d/%d, want %d/%d", got.ChunkRadius, got.MaxChunkRadius, test.want, test.want)
			}
		})
	}
}

func TestTransferRejectsIncompatibleBlockNetworkEncoding(t *testing.T) {
	initial := &fakeBackend{data: minecraft.GameData{UseBlockNetworkIDHashes: false}}
	replacement := &fakeBackend{data: minecraft.GameData{UseBlockNetworkIDHashes: true}}
	s := newSession(
		&Proxy{cfg: Config{Dial: func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error) {
			return replacement, nil
		}}},
		NopHandler{}, &fakeClient{}, initial, login.IdentityData{}, login.ClientData{}, "",
	)

	committed, err := s.transfer(context.Background(), "127.0.0.1:19133")
	if err == nil {
		t.Fatal("transfer accepted a backend with incompatible block-network encoding")
	}
	if committed {
		t.Fatal("incompatible backend transfer was committed")
	}
	backend, _ := s.currentBackend()
	if backend != initial {
		t.Fatalf("current backend = %#v, want initial backend", backend)
	}
}

func TestBackendReadFailureFallsBackToRemoteAddress(t *testing.T) {
	firstReadErr := errors.New("primary backend closed")
	secondReadErr := errors.New("fallback backend closed")
	primary := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		readErr: firstReadErr,
	}
	fallback := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 11, EntityUniqueID: 12},
		readErr: secondReadErr,
	}
	client := &fakeClient{addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 19132}}
	var addresses []string
	proxy := &Proxy{cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(_ context.Context, address string, _ login.IdentityData, _ login.ClientData, _ string) (Backend, error) {
			addresses = append(addresses, address)
			if len(addresses) == 1 {
				return fallback, nil
			}
			return nil, errors.New("fallback unavailable")
		},
	}}
	s := &session{
		proxy: proxy, handler: NopHandler{}, client: client, backend: primary,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}

	err := s.backendLoop(context.Background())
	if !errors.Is(err, secondReadErr) {
		t.Fatalf("backendLoop() error = %v, want fallback read error %v", err, secondReadErr)
	}
	if len(addresses) != 2 {
		t.Fatalf("fallback dial attempts = %d, want 2", len(addresses))
	}
	for _, address := range addresses {
		if address != proxy.cfg.RemoteAddress {
			t.Fatalf("fallback address = %q, want %q", address, proxy.cfg.RemoteAddress)
		}
	}
	backend, _ := s.currentBackend()
	if backend != fallback {
		t.Fatalf("current backend = %#v, want successful fallback %#v", backend, fallback)
	}
}

func TestBackendReadFailureBacksOffConsecutiveFallbacks(t *testing.T) {
	primary := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		readErr: errors.New("primary backend closed"),
	}
	fallback := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 11, EntityUniqueID: 12},
		readErr: errors.New("fallback backend closed"),
	}
	client := &fakeClient{addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 19132}}
	var attempts []time.Time
	proxy := &Proxy{cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error) {
			attempts = append(attempts, time.Now())
			if len(attempts) == 1 {
				return fallback, nil
			}
			return nil, errors.New("fallback unavailable")
		},
	}}
	s := &session{
		proxy: proxy, handler: NopHandler{}, client: client, backend: primary,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}

	if err := s.backendLoop(context.Background()); err == nil {
		t.Fatal("backendLoop() succeeded after fallback became unavailable")
	}
	if len(attempts) != 2 {
		t.Fatalf("fallback dial attempts = %d, want 2", len(attempts))
	}
	if elapsed := attempts[1].Sub(attempts[0]); elapsed < 50*time.Millisecond {
		t.Fatalf("consecutive fallback attempts were %v apart, want a backoff", elapsed)
	}
}

func TestFallbackPausesClientPacketsWhileDialing(t *testing.T) {
	primaryWrites := make(chan struct{}, 1)
	primary := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		readErr: errors.New("primary backend closed"),
		write: func(packet.Packet) error {
			primaryWrites <- struct{}{}
			return errors.New("primary backend closed")
		},
	}
	fallback := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 11, EntityUniqueID: 12},
		readErr: errors.New("fallback backend closed"),
	}
	client := &fakeClient{
		addr:      &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 19132},
		readQueue: make(chan packet.Packet, 1),
	}
	dialStarted, releaseDial := make(chan struct{}), make(chan struct{})
	var attempts int
	proxy := &Proxy{cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(_ context.Context, _ string, _ login.IdentityData, _ login.ClientData, _ string) (Backend, error) {
			attempts++
			if attempts != 1 {
				return nil, errors.New("fallback unavailable")
			}
			close(dialStarted)
			<-releaseDial
			return fallback, nil
		},
	}}
	s := &session{
		proxy: proxy, handler: NopHandler{}, client: client, backend: primary,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}
	backendDone := make(chan error, 1)
	go func() { backendDone <- s.backendLoop(context.Background()) }()
	<-dialStarted
	client.readQueue <- &packet.Text{Message: "during fallback"}
	close(client.readQueue)
	clientDone := make(chan error, 1)
	go func() { clientDone <- s.clientLoop() }()

	select {
	case <-primaryWrites:
		t.Fatal("client packet was written to dead backend during fallback dial")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseDial)
	select {
	case <-backendDone:
	case <-time.After(time.Second):
		t.Fatal("backend loop did not finish")
	}
	select {
	case <-clientDone:
	case <-time.After(time.Second):
		t.Fatal("client loop did not finish")
	}
}

func TestFallbackPausesClientPacketsDuringRetryBackoff(t *testing.T) {
	deadBackendWrites := make(chan struct{}, 1)
	fallback := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 11, EntityUniqueID: 12},
		readErr: errors.New("fallback backend closed"),
		write: func(packet.Packet) error {
			deadBackendWrites <- struct{}{}
			return errors.New("fallback backend closed")
		},
	}
	client := &fakeClient{
		addr:      &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 19132},
		readQueue: make(chan packet.Packet, 1),
	}
	proxy := &Proxy{cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error) {
			return nil, errors.New("fallback unavailable")
		},
	}}
	s := &session{
		proxy: proxy, handler: NopHandler{}, client: client, backend: fallback,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 11, backendUniqueID: 12,
		state: newBackendStateTracker(),
	}
	recoveryDone := make(chan error, 1)
	go func() {
		_, err := s.recoverFallback(context.Background(), 1)
		recoveryDone <- err
	}()
	deadline := time.Now().Add(time.Second)
	for s.routeMu.TryLock() {
		s.routeMu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("fallback recovery did not pause client routing")
		}
		time.Sleep(time.Millisecond)
	}
	client.readQueue <- &packet.Text{Message: "during retry backoff"}
	close(client.readQueue)
	clientDone := make(chan error, 1)
	go func() { clientDone <- s.clientLoop() }()

	select {
	case <-deadBackendWrites:
		t.Fatal("client packet was written to dead backend during retry backoff")
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case <-recoveryDone:
	case <-time.After(time.Second):
		t.Fatal("fallback recovery did not finish")
	}
	select {
	case <-clientDone:
	case <-time.After(time.Second):
		t.Fatal("client loop did not finish")
	}
}

func TestBackendDisconnectMessageExtractsPreLoginRejection(t *testing.T) {
	want := "You are not whitelisted on this server"
	kick := &net.OpError{
		Op:  "dial",
		Net: "minecraft",
		Err: &net.OpError{Op: want, Net: "minecraft", Err: net.ErrClosed},
	}
	err := errors.Join(errors.New("backup unavailable"), kick)
	got, ok := backendDisconnectMessage(err)
	if !ok || got != want {
		t.Fatalf("backendDisconnectMessage() = (%q, %t), want (%q, true)", got, ok, want)
	}
}

func TestBackendDisconnectMessageRejectsTransportFailure(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "minecraft", Err: net.ErrClosed}
	if got, ok := backendDisconnectMessage(err); ok {
		t.Fatalf("backendDisconnectMessage() = (%q, true), want no player-facing reason", got)
	}
}

func TestBackendRejectionDisconnectsClientWithoutFallback(t *testing.T) {
	const reason = "You are not whitelisted on this server"
	backend := &fakeBackend{
		data: minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		readErr: &net.OpError{
			Op:  "read packet",
			Net: "minecraft",
			Err: &net.OpError{Op: reason, Net: "minecraft", Err: net.ErrClosed},
		},
	}
	client := &fakeClient{addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 19132}}
	dialed := false
	proxy := &Proxy{cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error) {
			dialed = true
			return nil, errors.New("unexpected fallback")
		},
	}}
	s := &session{
		proxy: proxy, handler: NopHandler{}, client: client, backend: backend,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}

	if err := s.backendLoop(context.Background()); !errors.Is(err, backend.readErr) {
		t.Fatalf("backendLoop() error = %v, want backend rejection %v", err, backend.readErr)
	}
	if dialed {
		t.Fatal("backend rejection triggered fallback dialing")
	}
	if len(client.packets) != 1 {
		t.Fatalf("client packets = %d, want one disconnect", len(client.packets))
	}
	disconnect, ok := client.packets[0].(*packet.Disconnect)
	if !ok || disconnect.Message != reason {
		t.Fatalf("client packet = %#v, want disconnect message %q", client.packets[0], reason)
	}
}

func TestRunDoesNotReconnectAfterClientDisconnect(t *testing.T) {
	readStarted := make(chan struct{})
	releaseRead := make(chan struct{})
	backend := &blockingReadBackend{
		fakeBackend: fakeBackend{data: minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10}},
		started:     readStarted,
		release:     releaseRead,
	}
	client := &fakeClient{
		addr:      &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 19132},
		readQueue: make(chan packet.Packet),
	}
	dialed := make(chan struct{}, 1)
	proxy := &Proxy{cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error) {
			dialed <- struct{}{}
			return nil, errors.New("unexpected reconnect")
		},
	}}
	s := &session{
		proxy: proxy, handler: NopHandler{}, client: client, backend: backend,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}
	runDone := make(chan error, 1)
	go func() { runDone <- s.run(context.Background()) }()
	<-readStarted
	close(client.readQueue)
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("session did not stop after the client disconnected")
	}
	close(releaseRead)
	select {
	case <-dialed:
		t.Fatal("session reconnected to the fallback after the client disconnected")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestClientBatchLoopHonoursHandlerVerdicts(t *testing.T) {
	var backendPackets []packet.Packet
	backend := &fakeBackend{
		data: minecraft.GameData{EntityRuntimeID: 1, EntityUniqueID: 2},
		write: func(pk packet.Packet) error {
			backendPackets = append(backendPackets, pk)
			return nil
		},
	}
	client := &fakeClient{batchQueue: make(chan []packet.Packet, 1)}
	replacement := &packet.Text{Message: "replaced"}
	handler := &recordingHandler{onClientBatch: func(batch []*PacketContext) {
		for _, pkCtx := range batch {
			text, ok := pkCtx.Packet().(*packet.Text)
			if !ok {
				continue
			}
			switch text.Message {
			case "cancel me":
				pkCtx.Cancel()
			case "replace me":
				pkCtx.SetPacket(replacement)
			}
		}
	}}
	s := &session{
		proxy: &Proxy{batchReading: true}, handler: handler, client: client, backend: backend,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 1, backendUniqueID: 2,
		state: newBackendStateTracker(),
	}
	client.batchQueue <- []packet.Packet{
		&packet.Text{Message: "cancel me"},
		&packet.Text{Message: "replace me"},
		&packet.RequestChunkRadius{ChunkRadius: 8},
	}
	close(client.batchQueue)

	if err := s.clientLoop(); err == nil {
		t.Fatal("clientLoop() succeeded after the client closed")
	}
	if len(handler.clientBatchSizes) != 1 || handler.clientBatchSizes[0] != 3 {
		t.Fatalf("handler batch sizes = %v, want one batch of 3", handler.clientBatchSizes)
	}
	if len(backendPackets) != 2 {
		t.Fatalf("backend packets = %d, want the cancelled packet withheld", len(backendPackets))
	}
	if backendPackets[0] != packet.Packet(replacement) {
		t.Fatalf("backend packet = %#v, want the handler replacement", backendPackets[0])
	}
	if _, ok := backendPackets[1].(*packet.RequestChunkRadius); !ok {
		t.Fatalf("backend packet = %#v, want RequestChunkRadius", backendPackets[1])
	}
	if s.chunkRadius != 8 {
		t.Fatalf("session chunk radius = %d, want the batch-read request radius 8", s.chunkRadius)
	}
}

func TestBackendBatchLoopHonoursHandlerVerdicts(t *testing.T) {
	replacement := &packet.Text{Message: "replaced"}
	backend := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		readErr: errors.New("backend closed"),
		batches: [][]packet.Packet{{
			&packet.Text{Message: "cancel me"},
			&packet.Text{Message: "replace me"},
			&packet.Text{Message: "forward me"},
		}},
	}
	client := &fakeClient{}
	handler := &recordingHandler{onServerBatch: func(batch []*PacketContext) {
		for _, pkCtx := range batch {
			text, ok := pkCtx.Packet().(*packet.Text)
			if !ok {
				continue
			}
			switch text.Message {
			case "cancel me":
				pkCtx.Cancel()
			case "replace me":
				pkCtx.SetPacket(replacement)
			}
		}
	}}
	proxy := &Proxy{batchReading: true, cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error) {
			return nil, errors.New("fallback unavailable")
		},
	}}
	s := &session{
		proxy: proxy, handler: handler, client: client, backend: backend,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}

	if err := s.backendLoop(context.Background()); err == nil {
		t.Fatal("backendLoop() succeeded after the backend closed")
	}
	if len(client.packets) != 2 {
		t.Fatalf("client packets = %d, want the cancelled packet withheld", len(client.packets))
	}
	if client.packets[0] != packet.Packet(replacement) {
		t.Fatalf("client packet = %#v, want the handler replacement", client.packets[0])
	}
	text, ok := client.packets[1].(*packet.Text)
	if !ok || text.Message != "forward me" {
		t.Fatalf("client packet = %#v, want the forwarded text packet", client.packets[1])
	}
}

func TestBackendBatchLoopDropsRemainderAfterTransfer(t *testing.T) {
	primary := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		readErr: errors.New("primary backend closed"),
		batches: [][]packet.Packet{{
			&packet.Text{Message: "before transfer"},
			&packet.Transfer{Address: "127.0.0.1", Port: 19134},
			&packet.Text{Message: "after transfer"},
		}},
	}
	replacement := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 11, EntityUniqueID: 12},
		readErr: errors.New("replacement backend closed"),
	}
	client := &fakeClient{}
	var addresses []string
	proxy := &Proxy{batchReading: true, cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(_ context.Context, address string, _ login.IdentityData, _ login.ClientData, _ string) (Backend, error) {
			addresses = append(addresses, address)
			if len(addresses) == 1 {
				return replacement, nil
			}
			return nil, errors.New("fallback unavailable")
		},
	}}
	s := &session{
		proxy: proxy, handler: NopHandler{}, client: client, backend: primary,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}

	if err := s.backendLoop(context.Background()); err == nil {
		t.Fatal("backendLoop() succeeded after the fallback became unavailable")
	}
	if len(addresses) == 0 || addresses[0] != "127.0.0.1:19134" {
		t.Fatalf("dialed addresses = %v, want the transfer target first", addresses)
	}
	var before, after bool
	for _, pk := range client.packets {
		if text, ok := pk.(*packet.Text); ok {
			before = before || text.Message == "before transfer"
			after = after || text.Message == "after transfer"
		}
	}
	if !before || after {
		t.Fatalf("client received before=%t after=%t, want only packets preceding the transfer", before, after)
	}
	backend, _ := s.currentBackend()
	if backend != replacement {
		t.Fatalf("current backend = %#v, want the transfer replacement", backend)
	}
}

func TestBackendBatchLoopContinuesAfterFailedTransfer(t *testing.T) {
	primary := &fakeBackend{
		data:    minecraft.GameData{EntityRuntimeID: 9, EntityUniqueID: 10},
		readErr: errors.New("primary backend closed"),
		batches: [][]packet.Packet{{
			&packet.Text{Message: "before transfer"},
			&packet.Transfer{Address: "127.0.0.1", Port: 19134},
			&packet.Text{Message: "after transfer"},
		}},
	}
	client := &fakeClient{}
	handler := &recordingHandler{}
	proxy := &Proxy{batchReading: true, cfg: Config{
		RemoteAddress: "127.0.0.1:19133",
		Log:           slog.Default(),
		Dial: func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error) {
			return nil, errors.New("transfer target unavailable")
		},
	}}
	s := &session{
		proxy: proxy, handler: handler, client: client, backend: primary,
		clientRuntimeID: 1, clientUniqueID: 2,
		backendRuntimeID: 9, backendUniqueID: 10,
		state: newBackendStateTracker(),
	}

	if err := s.backendLoop(context.Background()); err == nil {
		t.Fatal("backendLoop() succeeded after the backend closed")
	}
	if len(handler.transferFailed) != 1 || handler.transferFailed[0] != "127.0.0.1:19134" {
		t.Fatalf("transfer failures = %v, want the transfer target", handler.transferFailed)
	}
	var before, after bool
	for _, pk := range client.packets {
		if text, ok := pk.(*packet.Text); ok {
			before = before || text.Message == "before transfer"
			after = after || text.Message == "after transfer"
		}
	}
	if !before || !after {
		t.Fatalf("client received before=%t after=%t, want the whole batch after a failed transfer", before, after)
	}
	backend, _ := s.currentBackend()
	if backend != primary {
		t.Fatalf("current backend = %#v, want the primary backend retained", backend)
	}
}

type fakeBackend struct {
	data     minecraft.GameData
	flushErr error
	readErr  error
	write    func(packet.Packet) error
	batches  [][]packet.Packet
}

type blockingReadBackend struct {
	fakeBackend
	started chan struct{}
	release chan struct{}
}

func (b *blockingReadBackend) ReadPacket() (packet.Packet, error) {
	close(b.started)
	<-b.release
	return nil, errors.New("backend closed")
}

type radiusHandler struct {
	NopHandler
	radius int32
}

func (h radiusHandler) ChunkRadius() int32 { return h.radius }

type recordingHandler struct {
	NopHandler
	clientBatchSizes []int
	transferFailed   []string
	onClientBatch    func([]*PacketContext)
	onServerBatch    func([]*PacketContext)
}

func (h *recordingHandler) HandleClientBatch(batch []*PacketContext) {
	h.clientBatchSizes = append(h.clientBatchSizes, len(batch))
	if h.onClientBatch != nil {
		h.onClientBatch(batch)
	}
}

func (h *recordingHandler) HandleServerBatch(batch []*PacketContext) {
	if h.onServerBatch != nil {
		h.onServerBatch(batch)
	}
}

func (h *recordingHandler) TransferFailed(address string, _ error) {
	h.transferFailed = append(h.transferFailed, address)
}

func (f *fakeBackend) GameData() minecraft.GameData       { return f.data }
func (f *fakeBackend) ReadPacket() (packet.Packet, error) { return nil, f.readErr }
func (f *fakeBackend) ReadBatch() ([]packet.Packet, error) {
	if len(f.batches) > 0 {
		batch := f.batches[0]
		f.batches = f.batches[1:]
		return batch, nil
	}
	return nil, f.readErr
}
func (f *fakeBackend) WritePacket(pk packet.Packet) error {
	if f.write != nil {
		return f.write(pk)
	}
	return nil
}
func (*fakeBackend) DoSpawn() error { return nil }
func (f *fakeBackend) Flush() error { return f.flushErr }
func (*fakeBackend) Close() error   { return nil }

type fakeClient struct {
	packets    []packet.Packet
	addr       net.Addr
	readQueue  chan packet.Packet
	batchQueue chan []packet.Packet
}

func (f *fakeClient) ReadPacket() (packet.Packet, error) {
	if f.readQueue != nil {
		if pk, ok := <-f.readQueue; ok {
			return pk, nil
		}
	}
	return nil, errors.New("unused")
}
func (f *fakeClient) ReadBatch() ([]packet.Packet, error) {
	if f.batchQueue != nil {
		if batch, ok := <-f.batchQueue; ok {
			return batch, nil
		}
	}
	return nil, errors.New("unused")
}
func (f *fakeClient) WritePacket(pk packet.Packet) error {
	f.packets = append(f.packets, pk)
	return nil
}
func (*fakeClient) StartGame(minecraft.GameData) error { return nil }
func (f *fakeClient) RemoteAddr() net.Addr             { return f.addr }
func (*fakeClient) Close() error                       { return nil }
