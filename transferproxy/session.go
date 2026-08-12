package proxy

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type clientConn interface {
	ReadPacket() (packet.Packet, error)
	ReadBatch() ([]packet.Packet, error)
	WritePacket(packet.Packet) error
	StartGame(minecraft.GameData) error
	RemoteAddr() net.Addr
	Close() error
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

	dispatchMu sync.Mutex
	starting   bool
	pending    []pendingPackets

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

type pendingPackets struct {
	pks   []packet.Packet
	batch bool
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
		state: newBackendStateTracker(), starting: true,
	}
}

func (s *session) start(ctx context.Context) error {
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
	if err := s.handler.Start(s.backend); err != nil {
		return err
	}

	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	s.starting = false
	pending := s.pending
	s.pending = nil
	for _, p := range pending {
		if err := s.dispatchServerPacketsLocked(ctx, p.pks, p.batch); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() { errCh <- s.backendLoop(ctx) }()
	if s.starting {
		startCh := make(chan error, 1)
		go func() { startCh <- s.start(ctx) }()
		select {
		case err := <-startCh:
			if err != nil {
				return err
			}
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	go func() { errCh <- s.clientLoop() }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *session) clientLoop() error {
	if s.proxy.batchReading {
		return s.clientBatchLoop()
	}
	for {
		pk, err := s.client.ReadPacket()
		if err != nil {
			return err
		}

		s.routeMu.Lock()
		s.rewriteClientPacket(pk)
		s.routeMu.Unlock()

		pkCtx := NewPacketContext(pk)
		s.handler.HandleClientPacket(pkCtx)
		if err := s.forwardClientPacket(pkCtx); err != nil {
			return err
		}
	}
}

func (s *session) clientBatchLoop() error {
	for {
		pks, err := s.client.ReadBatch()
		if err != nil {
			return err
		}

		batch := make([]*PacketContext, len(pks))
		s.routeMu.Lock()
		for i, pk := range pks {
			s.rewriteClientPacket(pk)
			batch[i] = NewPacketContext(pk)
		}
		s.routeMu.Unlock()

		s.handler.HandleClientBatch(batch)
		for _, pkCtx := range batch {
			if err := s.forwardClientPacket(pkCtx); err != nil {
				return err
			}
		}
	}
}

// forwardClientPacket writes the packet held by pkCtx to the backend unless the handler
// cancelled it.
func (s *session) forwardClientPacket(pkCtx *PacketContext) error {
	if pkCtx.Cancelled() {
		return nil
	}
	pk := pkCtx.Packet()
	if radius, ok := pk.(*packet.RequestChunkRadius); ok {
		s.chunkRadius = radius.ChunkRadius
	}
	return s.writeBackend(pk)
}

func (s *session) backendLoop(ctx context.Context) error {
	if s.proxy.batchReading {
		return s.backendBatchLoop(ctx)
	}
	consecutiveReadFailures := 0
	for {
		backend, generation := s.currentBackend()
		pk, err := backend.ReadPacket()
		if err != nil {
			consecutiveReadFailures, err = s.recoverBackendRead(ctx, backend, generation, consecutiveReadFailures, err)
			if err != nil {
				return err
			}
			continue
		}
		if !s.isCurrent(backend, generation) {
			continue
		}
		consecutiveReadFailures = 0
		if err := s.dispatchServerPackets(ctx, []packet.Packet{pk}, false); err != nil {
			return err
		}
	}
}

func (s *session) backendBatchLoop(ctx context.Context) error {
	consecutiveReadFailures := 0
	for {
		backend, generation := s.currentBackend()
		pks, err := backend.ReadBatch()
		if err != nil {
			consecutiveReadFailures, err = s.recoverBackendRead(ctx, backend, generation, consecutiveReadFailures, err)
			if err != nil {
				return err
			}
			continue
		}
		if !s.isCurrent(backend, generation) {
			continue
		}
		consecutiveReadFailures = 0
		if err := s.dispatchServerPackets(ctx, pks, true); err != nil {
			return err
		}
	}
}

func (s *session) dispatchServerPackets(ctx context.Context, pks []packet.Packet, batch bool) error {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	if s.starting {
		s.pending = append(s.pending, pendingPackets{pks: pks, batch: batch})
		return nil
	}
	return s.dispatchServerPacketsLocked(ctx, pks, batch)
}

func (s *session) dispatchServerPacketsLocked(ctx context.Context, pks []packet.Packet, batch bool) error {
	if !batch {
		for _, pk := range pks {
			if transfer, ok := pk.(*packet.Transfer); ok {
				if _, err := s.handleTransfer(ctx, transfer); err != nil {
					return err
				}
				continue
			}
			s.routeMu.Lock()
			pkCtx := NewPacketContext(pk)
			s.handler.HandleServerPacket(pkCtx)
			err := s.forwardServerPacketLocked(pkCtx)
			s.routeMu.Unlock()
			if err != nil {
				return err
			}
		}
		return nil
	}

	start := 0
	for idx, pk := range pks {
		transfer, ok := pk.(*packet.Transfer)
		if !ok {
			continue
		}
		if err := s.forwardServerBatch(pks[start:idx]); err != nil {
			return err
		}
		start = idx + 1
		transferred, err := s.handleTransfer(ctx, transfer)
		if err != nil {
			return err
		}
		if transferred {
			// The remainder of the batch was sent by the replaced backend. Drop it, just
			// like stale single-packet reads are dropped by the generation check.
			start = len(pks)
			break
		}
	}
	return s.forwardServerBatch(pks[start:])
}

// forwardServerBatch runs a batch of backend packets through the handler and forwards those that
// were not cancelled or suppressed to the client.
func (s *session) forwardServerBatch(pks []packet.Packet) error {
	if len(pks) == 0 {
		return nil
	}
	batch := make([]*PacketContext, len(pks))
	for i, pk := range pks {
		batch[i] = NewPacketContext(pk)
	}
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	s.handler.HandleServerBatch(batch)
	for _, pkCtx := range batch {
		if err := s.forwardServerPacketLocked(pkCtx); err != nil {
			return err
		}
	}
	return nil
}

// forwardServerPacketLocked rewrites and writes the packet held by pkCtx to the client unless the
// handler cancelled it. It must be called with routeMu held.
func (s *session) forwardServerPacketLocked(pkCtx *PacketContext) error {
	if pkCtx.Cancelled() {
		return nil
	}
	pk := pkCtx.Packet()
	if !s.rewriteServerPacket(pk) {
		return nil
	}
	s.state.handle(pk, s.clientRuntimeID)
	return s.client.WritePacket(pk)
}

// recoverBackendRead handles a failed backend read. It forwards backend rejections to the client
// and otherwise dials the fallback, returning the updated consecutive failure count or an error
// that ends the session.
func (s *session) recoverBackendRead(ctx context.Context, backend Backend, generation uint64, consecutiveReadFailures int, readErr error) (int, error) {
	if !s.isCurrent(backend, generation) {
		return consecutiveReadFailures, nil
	}
	if reason, rejected := backendDisconnectMessage(readErr); rejected {
		if writeErr := s.client.WritePacket(&packet.Disconnect{Message: reason, FilteredMessage: reason}); writeErr != nil {
			return consecutiveReadFailures, fmt.Errorf("proxy: forward backend disconnect: %w", writeErr)
		}
		return consecutiveReadFailures, readErr
	}
	if s.isStarting() {
		return consecutiveReadFailures, readErr
	}
	committed, fallbackErr := s.recoverFallback(ctx, consecutiveReadFailures)
	if fallbackErr != nil {
		return consecutiveReadFailures, fmt.Errorf("proxy: backend read failed: %w; fallback failed after commit %t: %v", readErr, committed, fallbackErr)
	}
	s.proxy.cfg.Log.Warn("backend connection lost; transferred to fallback", "address", s.proxy.cfg.RemoteAddress, "err", readErr)
	return consecutiveReadFailures + 1, nil
}

// handleTransfer performs the backend handoff requested by transfer. It reports whether the
// session now runs on the replacement backend; a failed transfer that did not commit leaves the
// current backend in place without an error.
func (s *session) handleTransfer(ctx context.Context, transfer *packet.Transfer) (bool, error) {
	address := net.JoinHostPort(transfer.Address, fmt.Sprint(transfer.Port))
	committed, err := s.transfer(ctx, address)
	if err == nil {
		return true, nil
	}
	if committed {
		return false, fmt.Errorf("proxy: committed transfer to %s failed synchronization: %w", address, err)
	}
	s.proxy.cfg.Log.Warn("backend transfer failed", "address", address, "err", err)
	s.handler.TransferFailed(address, err)
	return false, nil
}

func (s *session) isStarting() bool {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	return s.starting
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
