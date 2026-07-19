// Package proxy provides a standalone Minecraft Bedrock RakNet proxy.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	initialFallbackRetryDelay = 100 * time.Millisecond
	maximumFallbackRetryDelay = 2 * time.Second
)

// Config configures a standalone Bedrock proxy.
type Config struct {
	LocalAddress  string
	RemoteAddress string
	Log           *slog.Logger
	Listen        minecraft.ListenConfig
	DialTimeout   time.Duration
	NewHandler    HandlerFactory

	// Dial may be set by advanced users to customise backend connections.
	Dial DialFunc
}

// DialFunc establishes and logs into a backend, stopping before DoSpawn.
type DialFunc func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error)

// Backend is the backend connection surface required by the native proxy.
type Backend interface {
	GameData() minecraft.GameData
	WritePacket(packet.Packet) error
	Close() error
	ReadPacket() (packet.Packet, error)
	DoSpawn() error
	Flush() error
}

// HandlerFactory creates the optional packet and backend lifecycle handler for a client.
type HandlerFactory func(HandlerContext) Handler

// HandlerContext contains the connection state available while constructing a Handler.
type HandlerContext struct {
	Client   *minecraft.Conn
	Listener *minecraft.Listener
	Log      *slog.Logger
}

// Handler integrates optional processing with the proxy session. Returning false from either
// packet method prevents that packet from being forwarded.
type Handler interface {
	Start(Backend) error
	HandleClientPacket(*packet.Packet) bool
	HandleServerPacket(*packet.Packet) bool
	TransferBackend(Backend) error
	TransferFailed(string, error)
	Close() error
}

// ChunkRadiusProvider overrides the client-requested chunk radius used when a transfer asks the
// replacement backend to begin sending chunks.
type ChunkRadiusProvider interface {
	ChunkRadius() int32
}

// NopHandler forwards every packet and accepts every backend lifecycle event. Embed it in handlers
// that only need to override part of Handler.
type NopHandler struct{}

func (NopHandler) Start(Backend) error                    { return nil }
func (NopHandler) HandleClientPacket(*packet.Packet) bool { return true }
func (NopHandler) HandleServerPacket(*packet.Packet) bool { return true }
func (NopHandler) TransferBackend(Backend) error          { return nil }
func (NopHandler) TransferFailed(string, error)           {}
func (NopHandler) Close() error                           { return nil }

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

// Listen starts a standalone Bedrock proxy.
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
	if cfg.Listen.Compression == nil {
		cfg.Listen.Compression = packet.SnappyCompression
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
		reason := "Unable to connect to the backend server."
		if backendReason, ok := backendDisconnectMessage(err); ok {
			reason = backendReason
		}
		_ = p.listener.Disconnect(conn, reason)
		return err
	}
	handler := Handler(NopHandler{})
	if p.cfg.NewHandler != nil {
		handler = p.cfg.NewHandler(HandlerContext{
			Client: conn, Listener: p.listener,
			Log: p.cfg.Log.With("player", conn.IdentityData().DisplayName),
		})
		if handler == nil {
			handler = NopHandler{}
		}
	}
	s := newSession(p, handler, conn, backend, conn.IdentityData(), clientData, conn.RemoteAddr().String())
	defer s.close()
	if err := s.start(); err != nil {
		return err
	}
	return s.run(ctx)
}

func backendDisconnectMessage(err error) (string, bool) {
	var disconnect minecraft.DisconnectError
	if errors.As(err, &disconnect) && disconnect != "" {
		return disconnect.Error(), true
	}
	return legacyBackendDisconnectMessage(err)
}

func legacyBackendDisconnectMessage(err error) (string, bool) {
	if opErr, ok := err.(*net.OpError); ok {
		if cause, ok := opErr.Err.(*net.OpError); ok && errors.Is(cause.Err, net.ErrClosed) && playerFacingDisconnectOperation(cause.Op) {
			return cause.Op, true
		}
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range joined.Unwrap() {
			if message, ok := legacyBackendDisconnectMessage(cause); ok {
				return message, true
			}
		}
		return "", false
	}
	if cause := errors.Unwrap(err); cause != nil {
		return legacyBackendDisconnectMessage(cause)
	}
	return "", false
}

func playerFacingDisconnectOperation(operation string) bool {
	switch operation {
	case "", "accept", "close", "dial", "do spawn", "flush", "read", "read packet", "start game", "write", "write packet":
		return false
	default:
		return true
	}
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
	return func(ctx context.Context, address string, identity login.IdentityData, client login.ClientData, _ string) (Backend, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return minecraft.Dialer{
			IdentityData:        identity,
			ClientData:          client,
			KeepXBLIdentityData: true,
		}.DialContext(ctx, "raknet", address)
	}
}

type session struct {
	proxy   *Proxy
	handler Handler
	client  clientConn

	routeMu    sync.Mutex
	backMu     sync.RWMutex
	backend    Backend
	generation uint64
	transferMu sync.Mutex

	clientRuntimeID      uint64
	clientUniqueID       int64
	clientDimension      int32
	blockNetworkIDHashes bool
	backendRuntimeID     uint64
	backendUniqueID      int64
	chunkRadius          int32
	identity             login.IdentityData
	clientData           login.ClientData
	clientAddress        string
	state                *backendStateTracker
}

func newSession(proxy *Proxy, handler Handler, client clientConn, backend Backend, identity login.IdentityData, clientData login.ClientData, clientAddress string) *session {
	data := backend.GameData()
	return &session{
		proxy: proxy, handler: handler, client: client, backend: backend,
		clientRuntimeID: data.EntityRuntimeID, clientUniqueID: data.EntityUniqueID,
		clientDimension: data.Dimension, blockNetworkIDHashes: data.UseBlockNetworkIDHashes,
		backendRuntimeID: data.EntityRuntimeID,
		backendUniqueID:  data.EntityUniqueID, chunkRadius: data.ChunkRadius,
		identity: identity, clientData: clientData, clientAddress: clientAddress,
		state: newBackendStateTracker(),
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
	return s.handler.Start(s.backend)
}

func (s *session) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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
		if s.handler.HandleClientPacket(&pk) {
			if radius, ok := pk.(*packet.RequestChunkRadius); ok {
				s.chunkRadius = radius.ChunkRadius
			}
			err = s.writeBackend(pk)
		}
		s.routeMu.Unlock()
		if err != nil {
			return err
		}
	}
}

func (s *session) backendLoop(ctx context.Context) error {
	consecutiveReadFailures := 0
	for {
		backend, generation := s.currentBackend()
		pk, err := backend.ReadPacket()
		if err != nil {
			if s.isCurrent(backend, generation) {
				if reason, rejected := backendDisconnectMessage(err); rejected {
					if writeErr := s.client.WritePacket(&packet.Disconnect{Message: reason, FilteredMessage: reason}); writeErr != nil {
						return fmt.Errorf("proxy: forward backend disconnect: %w", writeErr)
					}
					return err
				}
				committed, fallbackErr := s.recoverFallback(ctx, consecutiveReadFailures)
				if fallbackErr != nil {
					return fmt.Errorf("proxy: backend read failed: %w; fallback failed after commit %t: %v", err, committed, fallbackErr)
				}
				consecutiveReadFailures++
				s.proxy.cfg.Log.Warn("backend connection lost; transferred to fallback", "address", s.proxy.cfg.RemoteAddress, "err", err)
			}
			continue
		}
		if !s.isCurrent(backend, generation) {
			continue
		}
		consecutiveReadFailures = 0
		if transfer, ok := pk.(*packet.Transfer); ok {
			address := net.JoinHostPort(transfer.Address, fmt.Sprint(transfer.Port))
			committed, err := s.transfer(ctx, address)
			if err != nil && committed {
				return fmt.Errorf("proxy: committed transfer to %s failed synchronization: %w", address, err)
			}
			if err != nil {
				s.proxy.cfg.Log.Warn("backend transfer failed", "address", address, "err", err)
				s.handler.TransferFailed(address, err)
			}
			continue
		}
		s.routeMu.Lock()
		if s.handler.HandleServerPacket(&pk) {
			if s.rewriteServerPacket(pk) {
				s.state.handle(pk, s.clientRuntimeID)
				err = s.client.WritePacket(pk)
			}
		}
		s.routeMu.Unlock()
		if err != nil {
			return err
		}
	}
}

func fallbackRetryDelay(consecutiveFailures int) time.Duration {
	delay := initialFallbackRetryDelay
	for i := 1; i < consecutiveFailures && delay < maximumFallbackRetryDelay; i++ {
		delay = min(delay*2, maximumFallbackRetryDelay)
	}
	return delay
}

func waitForFallbackRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *session) transfer(ctx context.Context, address string) (bool, error) {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	return s.transferLocked(ctx, address)
}

func (s *session) recoverFallback(ctx context.Context, consecutiveFailures int) (bool, error) {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	if consecutiveFailures > 0 {
		if err := waitForFallbackRetry(ctx, fallbackRetryDelay(consecutiveFailures)); err != nil {
			return false, err
		}
	}
	return s.transferLocked(ctx, s.proxy.cfg.RemoteAddress)
}

func (s *session) transferLocked(ctx context.Context, address string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	backend, err := s.proxy.cfg.Dial(ctx, address, s.identity, s.clientData, s.clientAddress)
	if err != nil {
		return false, err
	}
	backendUsesHashes := backend.GameData().UseBlockNetworkIDHashes
	if backendUsesHashes != s.blockNetworkIDHashes {
		_ = backend.Close()
		return false, fmt.Errorf("proxy: backend block-hash setting %t does not match session setting %t", backendUsesHashes, s.blockNetworkIDHashes)
	}
	if err := backend.DoSpawn(); err != nil {
		_ = backend.Close()
		return false, err
	}

	if err := s.handler.TransferBackend(backend); err != nil {
		_ = backend.Close()
		return false, err
	}
	old := s.swapBackend(backend)
	err = s.resetTransferState()
	_ = old.Close()
	return true, err
}

func (s *session) resetTransferState() error {
	for _, pk := range s.state.clearPackets(s.clientRuntimeID) {
		if err := s.client.WritePacket(pk); err != nil {
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
	radius := s.chunkRadius
	if provider, ok := s.handler.(ChunkRadiusProvider); ok {
		radius = provider.ChunkRadius()
	}
	radius = min(max(radius, 1), 255)
	if err := s.backend.WritePacket(&packet.RequestChunkRadius{ChunkRadius: radius, MaxChunkRadius: uint8(radius)}); err != nil {
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
	packets := make([]packet.Packet, 0, 12)
	packets = append(packets,
		&packet.StopSound{StopAll: true},
		&packet.LevelEvent{EventType: packet.LevelEventStopRaining, EventData: 10_000},
		&packet.LevelEvent{EventType: packet.LevelEventStopThunderstorm},
	)
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
		&packet.SetDifficulty{Difficulty: uint32(data.Difficulty)},
		&packet.GameRulesChanged{GameRules: data.GameRules},
		&packet.MovePlayer{EntityRuntimeID: data.EntityRuntimeID, Position: data.PlayerPosition, Pitch: data.Pitch, Yaw: data.Yaw, HeadYaw: data.Yaw, Mode: packet.MoveModeReset},
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
	data := backend.GameData()
	s.backendRuntimeID = data.EntityRuntimeID
	s.backendUniqueID = data.EntityUniqueID
	s.generation++
	return old
}

func (s *session) writeBackend(pk packet.Packet) error {
	s.backMu.RLock()
	defer s.backMu.RUnlock()
	return s.backend.WritePacket(pk)
}

func (s *session) close() {
	_ = s.handler.Close()
	backend, _ := s.currentBackend()
	_ = backend.Close()
	_ = s.client.Close()
}

func (s *session) rewriteClientPacket(pk packet.Packet) {
	from, to := s.clientRuntimeID, s.backendRuntimeID
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
		if pk.TargetEntityRuntimeID == math.MaxInt64 {
			pk.TargetEntityRuntimeID = from
		} else if pk.TargetEntityRuntimeID == from {
			pk.TargetEntityRuntimeID = to
		}
	case *packet.InventoryTransaction:
		if tx, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			if tx.TargetEntityRuntimeID == math.MaxInt64 {
				tx.TargetEntityRuntimeID = from
			} else if tx.TargetEntityRuntimeID == from {
				tx.TargetEntityRuntimeID = to
			}
		}
	case *packet.ContainerOpen:
		if pk.ContainerEntityUniqueID == s.clientUniqueID {
			pk.ContainerEntityUniqueID = s.backendUniqueID
		}
	}
}

func (s *session) rewriteServerPacket(pk packet.Packet) bool {
	from, to := s.backendRuntimeID, s.clientRuntimeID
	rewriteRuntimeID := func(id *uint64) {
		if *id == from {
			*id = to
		} else if from != to && *id == to {
			*id = math.MaxInt64
		}
	}
	rewriteUniqueID := func(id *int64) {
		if *id == s.backendUniqueID {
			*id = s.clientUniqueID
		} else if s.backendUniqueID != s.clientUniqueID && *id == s.clientUniqueID {
			*id = math.MaxInt64
		}
	}
	switch pk := pk.(type) {
	case *packet.AddPlayer:
		if pk.EntityRuntimeID == from {
			return false
		}
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.AbilityData.EntityUniqueID)
	case *packet.AddActor:
		if pk.EntityRuntimeID == from {
			return false
		}
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.EntityUniqueID)
	case *packet.AddItemActor:
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.EntityUniqueID)
	case *packet.AddPainting:
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.EntityUniqueID)
	case *packet.MoveActorAbsolute:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MovePlayer:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MobEquipment:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.Animate:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.ActorEvent:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.PlayerAction:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.SetActorData:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.SetActorMotion:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.UpdateAttributes:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MobEffect:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.Respawn:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MobArmourEquipment:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.UpdateAbilities:
		rewriteUniqueID(&pk.AbilityData.EntityUniqueID)
	case *packet.ContainerOpen:
		rewriteUniqueID(&pk.ContainerEntityUniqueID)
	case *packet.RemoveActor:
		rewriteUniqueID(&pk.EntityUniqueID)
	}
	return true
}
