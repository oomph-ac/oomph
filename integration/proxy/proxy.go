// Package proxy provides Oomph's native standalone RakNet proxy integration.
package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	playercontext "github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Config configures a standalone Oomph proxy.
type Config struct {
	LocalAddress  string
	RemoteAddress string
	Log           *slog.Logger
	Listen        minecraft.ListenConfig
	DialTimeout   time.Duration
	Configure     func(*player.Player)

	// Dial may be set by advanced users to customise backend connections.
	Dial DialFunc
}

// DialFunc establishes and logs into a backend, stopping before DoSpawn.
type DialFunc func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error)

// Backend is the backend connection surface required by the native proxy.
type Backend interface {
	player.ServerConn
	ReadPacket() (packet.Packet, error)
	DoSpawn() error
	Flush() error
}

type clientConn interface {
	ReadPacket() (packet.Packet, error)
	WritePacket(packet.Packet) error
	StartGame(minecraft.GameData) error
	RemoteAddr() net.Addr
	Close() error
}

// Proxy accepts Bedrock clients and keeps them connected while their backend
// connection is replaced during packet.Transfer handoffs.
type Proxy struct {
	cfg      Config
	listener *minecraft.Listener
	done     chan struct{}
	closeOne sync.Once
}

// Listen starts a native Oomph proxy.
func Listen(ctx context.Context, cfg Config) (*Proxy, error) {
	if cfg.LocalAddress == "" || cfg.RemoteAddress == "" {
		return nil, fmt.Errorf("proxy: local and remote addresses are required")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.Dial == nil {
		cfg.Dial = defaultDial(cfg.DialTimeout)
	}
	l, err := cfg.Listen.Listen("raknet", cfg.LocalAddress)
	if err != nil {
		return nil, fmt.Errorf("proxy: listen: %w", err)
	}
	p := &Proxy{cfg: cfg, listener: l, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = p.Close()
		case <-p.done:
		}
	}()
	return p, nil
}

// Serve accepts clients until the proxy is closed.
func (p *Proxy) Serve(ctx context.Context) error {
	for {
		raw, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			default:
				return fmt.Errorf("proxy: accept: %w", err)
			}
		}
		conn := raw.(*minecraft.Conn)
		go func() {
			if err := p.serveClient(ctx, conn); err != nil {
				p.cfg.Log.Error("proxy session closed", "player", conn.IdentityData().DisplayName, "err", err)
			}
		}()
	}
}

func (p *Proxy) serveClient(ctx context.Context, conn *minecraft.Conn) error {
	clientData := conn.ClientData()
	clientData.ThirdPartyName = conn.IdentityData().DisplayName
	backend, err := p.cfg.Dial(ctx, p.cfg.RemoteAddress, conn.IdentityData(), clientData, conn.RemoteAddr().String())
	if err != nil {
		_ = p.listener.Disconnect(conn, "Unable to connect to the backend server.")
		return err
	}
	pl := player.New(p.cfg.Log.With("player", conn.IdentityData().DisplayName), player.MonitoringState{CurrentTime: time.Now()}, p.listener)
	component.Register(pl)
	detection.Register(pl)
	pl.SetConn(conn)
	if p.cfg.Configure != nil {
		p.cfg.Configure(pl)
	}
	s := newSession(p, pl, conn, backend)
	defer s.close()
	if err := s.start(); err != nil {
		return err
	}
	return s.run(ctx)
}

// Close stops accepting new clients.
func (p *Proxy) Close() error {
	var err error
	p.closeOne.Do(func() {
		close(p.done)
		err = p.listener.Close()
	})
	return err
}

func defaultDial(timeout time.Duration) DialFunc {
	return func(_ context.Context, address string, identity login.IdentityData, client login.ClientData, _ string) (Backend, error) {
		return minecraft.Dialer{
			IdentityData: identity,
			ClientData:   client,
			FlushRate:    -1,
		}.DialTimeout("raknet", address, timeout)
	}
}

type session struct {
	proxy  *Proxy
	player *player.Player
	client clientConn

	routeMu    sync.Mutex
	backMu     sync.RWMutex
	backend    Backend
	generation uint64
	transferMu sync.Mutex

	clientRuntimeID uint64
	clientUniqueID  int64
	clientDimension int32
}

func newSession(proxy *Proxy, p *player.Player, client clientConn, backend Backend) *session {
	data := backend.GameData()
	return &session{
		proxy: proxy, player: p, client: client, backend: backend,
		clientRuntimeID: data.EntityRuntimeID, clientUniqueID: data.EntityUniqueID,
		clientDimension: data.Dimension,
	}
}

func (s *session) start() error {
	data := s.backend.GameData()
	data.PlayerMovementSettings.RewindHistorySize = 100
	errCh := make(chan error, 2)
	go func() { errCh <- s.client.StartGame(data) }()
	go func() { errCh <- s.backend.DoSpawn() }()
	for range 2 {
		if err := <-errCh; err != nil {
			return err
		}
	}
	s.player.SetServerConn(s.backend)
	go s.player.StartTicking()
	return nil
}

func (s *session) run(ctx context.Context) error {
	errCh := make(chan error, 2)
	go func() { errCh <- s.clientLoop() }()
	go func() { errCh <- s.backendLoop(ctx) }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *session) clientLoop() error {
	for {
		pk, err := s.client.ReadPacket()
		if err != nil {
			return err
		}
		s.routeMu.Lock()
		s.rewriteClientPacket(pk)
		ctx := playercontext.NewHandlePacketContext(&pk)
		s.player.HandleClientPacket(ctx)
		if !ctx.Cancelled() {
			err = s.writeBackend(*ctx.Packet())
		}
		s.routeMu.Unlock()
		if err != nil {
			return err
		}
	}
}

func (s *session) backendLoop(ctx context.Context) error {
	for {
		backend, generation := s.currentBackend()
		pk, err := backend.ReadPacket()
		if err != nil {
			if s.isCurrent(backend, generation) {
				return err
			}
			continue
		}
		if !s.isCurrent(backend, generation) {
			continue
		}
		if transfer, ok := pk.(*packet.Transfer); ok {
			address := net.JoinHostPort(transfer.Address, fmt.Sprint(transfer.Port))
			committed, err := s.transfer(ctx, address)
			if err != nil && committed {
				return fmt.Errorf("proxy: committed transfer to %s failed synchronization: %w", address, err)
			}
			if err != nil {
				s.proxy.cfg.Log.Warn("backend transfer failed", "address", address, "err", err)
				s.player.Message("<red>Unable to transfer to %s.</red>", address)
			}
			continue
		}
		s.routeMu.Lock()
		ctx := playercontext.NewHandlePacketContext(&pk)
		s.player.HandleServerPacket(ctx)
		if !ctx.Cancelled() {
			s.rewriteServerPacket(*ctx.Packet())
			err = s.client.WritePacket(*ctx.Packet())
		}
		s.routeMu.Unlock()
		if err != nil {
			return err
		}
	}
}

func (s *session) transfer(ctx context.Context, address string) (bool, error) {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	backend, err := s.proxy.cfg.Dial(ctx, address, s.player.IdentityDat, s.player.ClientDat, s.client.RemoteAddr().String())
	if err != nil {
		return false, err
	}
	if err := backend.DoSpawn(); err != nil {
		_ = backend.Close()
		return false, err
	}

	s.routeMu.Lock()
	old := s.swapBackend(backend)
	state := s.player.TransferServerConn(backend)
	err = s.resetTransferState(state)
	s.routeMu.Unlock()
	_ = old.Close()
	return true, err
}

func (s *session) resetTransferState(state player.BackendTransferState) error {
	for _, effectID := range state.EffectIDs {
		if err := s.client.WritePacket(&packet.MobEffect{EntityRuntimeID: s.clientRuntimeID, Operation: packet.MobEffectRemove, EffectType: effectID}); err != nil {
			return err
		}
	}
	data := s.backend.GameData()
	for _, pk := range transferResetPackets(s.clientDimension, data) {
		s.rewriteServerPacket(pk)
		if err := s.client.WritePacket(pk); err != nil {
			return err
		}
	}
	s.clientDimension = data.Dimension
	radius := s.player.WorldUpdater().ChunkRadius()
	maxRadius := min(max(radius, 0), 255)
	if err := s.backend.WritePacket(&packet.RequestChunkRadius{ChunkRadius: radius, MaxChunkRadius: uint8(maxRadius)}); err != nil {
		return err
	}
	return s.backend.Flush()
}

func transferResetPackets(currentDimension int32, data minecraft.GameData) []packet.Packet {
	fakeDimension := int32(packet.DimensionOverworld)
	for _, candidate := range []int32{packet.DimensionOverworld, packet.DimensionNether, packet.DimensionEnd} {
		if candidate != currentDimension && candidate != data.Dimension {
			fakeDimension = candidate
			break
		}
	}
	packets := make([]packet.Packet, 0, 8)
	if currentDimension == data.Dimension {
		packets = append(packets,
			&packet.ChangeDimension{Dimension: fakeDimension, Position: data.PlayerPosition},
			&packet.PlayStatus{Status: packet.PlayStatusPlayerSpawn},
			&packet.PlayerAction{EntityRuntimeID: data.EntityRuntimeID, ActionType: protocol.PlayerActionDimensionChangeDone},
		)
	}
	packets = append(packets,
		&packet.ChangeDimension{Dimension: data.Dimension, Position: data.PlayerPosition},
		&packet.PlayStatus{Status: packet.PlayStatusPlayerSpawn},
		&packet.PlayerAction{EntityRuntimeID: data.EntityRuntimeID, ActionType: protocol.PlayerActionDimensionChangeDone},
		&packet.SetPlayerGameType{GameType: data.PlayerGameMode},
		&packet.GameRulesChanged{GameRules: data.GameRules},
		&packet.MovePlayer{EntityRuntimeID: data.EntityRuntimeID, Position: data.PlayerPosition, Mode: packet.MoveModeReset},
	)
	return packets
}

func (s *session) currentBackend() (Backend, uint64) {
	s.backMu.RLock()
	defer s.backMu.RUnlock()
	return s.backend, s.generation
}

func (s *session) isCurrent(_ Backend, generation uint64) bool {
	s.backMu.RLock()
	defer s.backMu.RUnlock()
	return s.generation == generation
}

func (s *session) swapBackend(backend Backend) Backend {
	s.backMu.Lock()
	defer s.backMu.Unlock()
	old := s.backend
	s.backend = backend
	s.generation++
	return old
}

func (s *session) writeBackend(pk packet.Packet) error {
	s.backMu.RLock()
	defer s.backMu.RUnlock()
	return s.backend.WritePacket(pk)
}

func (s *session) close() {
	_ = s.player.Close()
	backend, _ := s.currentBackend()
	_ = backend.Close()
	_ = s.client.Close()
}

func (s *session) rewriteClientPacket(pk packet.Packet) {
	from, to := s.clientRuntimeID, s.player.RuntimeId
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.MobEquipment:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Animate:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.PlayerAction:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Respawn:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Interact:
		if pk.TargetEntityRuntimeID == from {
			pk.TargetEntityRuntimeID = to
		}
	case *packet.InventoryTransaction:
		if tx, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && tx.TargetEntityRuntimeID == from {
			tx.TargetEntityRuntimeID = to
		}
	case *packet.ContainerOpen:
		if pk.ContainerEntityUniqueID == s.clientUniqueID {
			pk.ContainerEntityUniqueID = s.player.UniqueId
		}
	}
}

func (s *session) rewriteServerPacket(pk packet.Packet) {
	from, to := s.player.RuntimeId, s.clientRuntimeID
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.MobEquipment:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Animate:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.ActorEvent:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.PlayerAction:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.SetActorData:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.UpdateAttributes:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.MobEffect:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Respawn:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.MobArmourEquipment:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.UpdateAbilities:
		if pk.AbilityData.EntityUniqueID == s.player.UniqueId {
			pk.AbilityData.EntityUniqueID = s.clientUniqueID
		}
	case *packet.ContainerOpen:
		if pk.ContainerEntityUniqueID == s.player.UniqueId {
			pk.ContainerEntityUniqueID = s.clientUniqueID
		}
	case *packet.RemoveActor:
		if pk.EntityUniqueID == s.player.UniqueId {
			pk.EntityUniqueID = s.clientUniqueID
		}
	}
}
