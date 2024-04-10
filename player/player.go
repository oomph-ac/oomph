package player

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sasha-s/go-deadlock"
	"github.com/sirupsen/logrus"
)

const (
	GameVersion1_20_0  = 589
	GameVersion1_20_10 = 594
	GameVersion1_20_30 = 618
	GameVersion1_20_40 = 622
)

const TicksPerSecond = 20

const targetedProcessingDelay = 10 * time.Millisecond

type Player struct {
	MState MonitoringState

	// Connected is true if the player is connected to Oomph.
	Connected bool
	Closed    bool

	Alive bool
	Ready bool

	CloseChan chan bool
	CloseFunc sync.Once

	ClientPkFunc func([]packet.Packet) error
	ServerPkFunc func([]packet.Packet) error

	ClientDat   login.ClientData
	IdentityDat login.IdentityData
	GameDat     minecraft.GameData
	Version     int32

	// combatMode and movementMode are the authority modes of the player. They are used to determine
	// how certain actions should be handled.
	CombatMode   AuthorityMode
	MovementMode AuthorityMode

	// With fast transfers, the client will still retain it's original runtime and unique IDs, so
	// we must translate them to new ones, while still retaining the old ones for the client to use.
	RuntimeId, ClientRuntimeId uint64
	UniqueId, ClientUniqueId   int64
	IDModified                 bool

	// ClientTick is the tick of the client, synchronized with the server's on an interval.
	// ClientFrame is the simulation frame of the client, sent in PlayerAuthInput.
	ClientTick, ClientFrame int64
	ServerTick              int64
	Tps                     float32
	LastServerTick          time.Time

	// World is the world of the player.
	World *world.World

	// GameMode is the gamemode of the player. The player is exempt from movement predictions
	// if they are not in survival or adventure mode.
	GameMode int32

	// SentryTransaction is the transaction that is used to monitor performance of packet handling.
	SentryTransaction *sentry.Span

	// conn is the connection to the client, and serverConn is the connection to the server.
	conn, serverConn *minecraft.Conn
	// PacketQueue is a queue of client packets that are to be processed by the server. This is only used
	// in direct mode.
	PacketQueue []packet.Packet
	// ProcessMu is a mutex that is locked whenever packets need to be processed. It is used to
	// prevent race conditions, and to maintain accuracy with anti-cheat.
	// e.g - making sure all acknowledgements are sent in the same batch as the packets they are
	// being associated with.
	ProcessMu deadlock.Mutex

	// packetHandlers contains packet packetHandlers registered to the player.
	packetHandlers []Handler
	// detections contains packet handlers specifically used for detections.
	detections []Handler

	// eventHandler is a handler that handles events such as punishments and flags from detections.
	eventHandler EventHandler

	log *logrus.Logger

	// readingBatches is true if Oomph has been configured to read batches of packets from the client instead
	// of reading them one by one.
	readingBatches bool
}

// New creates and returns a new Player instance.
func New(log *logrus.Logger, readingBatches bool, mState MonitoringState) *Player {
	p := &Player{
		MState: mState,

		Connected: true,

		CombatMode:   AuthorityModeSemi,
		MovementMode: AuthorityModeSemi,

		World: world.New(),

		PacketQueue: []packet.Packet{},

		ClientTick:     0,
		ClientFrame:    0,
		ServerTick:     0,
		LastServerTick: time.Now(),
		Tps:            20.0,

		packetHandlers: []Handler{},
		detections:     []Handler{},

		eventHandler: &NopEventHandler{},

		readingBatches: readingBatches,

		log:       log,
		CloseChan: make(chan bool),
	}

	p.ClientPkFunc = p.DefaultHandleFromClient
	p.ServerPkFunc = p.DefaultHandleFromServer

	return p
}

// SetTime sets the current time of the player.
func (p *Player) SetTime(t time.Time) {
	p.MState.CurrentTime = t
}

// Time returns the current time of the player.
func (p *Player) Time() time.Time {
	if !p.MState.IsReplay {
		return time.Now()
	}

	return p.MState.CurrentTime
}

// SendPacketToClient sends a packet to the client.
func (p *Player) SendPacketToClient(pk packet.Packet) error {
	if p.MState.IsReplay {
		return nil
	}

	if p.conn == nil {
		return oerror.New("player connection is nil")
	}
	return p.conn.WritePacket(pk)
}

// SendPacketToServer sends a packet to the server.
func (p *Player) SendPacketToServer(pk packet.Packet) error {
	if p.MState.IsReplay {
		return nil
	}

	if p.serverConn == nil {
		// Don't return an error here, because the server connection may be nil if direct mode is used.
		return nil
	}
	return p.serverConn.WritePacket(pk)
}

// DefaultHandleFromClient handles a packet from the client.
func (p *Player) DefaultHandleFromClient(pks []packet.Packet) error {
	if p.Closed {
		return nil
	}

	p.SentryTransaction = sentry.StartTransaction(
		context.Background(),
		"oomph:handle_client",
		sentry.WithOpName("p.ClientPkFunc"),
		sentry.WithDescription("Handling packets from the client"),
	)
	defer func() {
		p.SentryTransaction.Status = sentry.SpanStatusOK
		if time.Since(p.SentryTransaction.StartTime) >= targetedProcessingDelay {
			p.SentryTransaction.Status = sentry.SpanStatusDeadlineExceeded
		}

		p.SentryTransaction.Finish()
	}()

	for _, pk := range pks {
		if err := p.handleOneFromClient(pk); err != nil {
			return err
		}
	}

	return nil
}

// DefaultHandleFromServer handles a packet from the server.
func (p *Player) DefaultHandleFromServer(pks []packet.Packet) error {
	if p.Closed {
		return nil
	}

	p.SentryTransaction = sentry.StartTransaction(
		context.Background(),
		"oomph:handle_server",
		sentry.WithOpName("p.ServerPkFunc"),
		sentry.WithDescription("Handling packets from the server"),
	)
	defer func() {
		p.SentryTransaction.Status = sentry.SpanStatusOK
		if time.Since(p.SentryTransaction.StartTime) >= targetedProcessingDelay {
			p.SentryTransaction.Status = sentry.SpanStatusDeadlineExceeded
		}

		p.SentryTransaction.Finish()
	}()

	for _, pk := range pks {
		if err := p.handleOneFromServer(pk); err != nil {
			return err
		}
	}

	return nil
}

func (p *Player) handleOneFromClient(pk packet.Packet) error {
	span := sentry.StartSpan(p.SentryTransaction.Context(), fmt.Sprintf("p.handleOneFromClient(%T)", pk))
	defer span.Finish()

	if s, ok := pk.(*packet.ScriptMessage); ok && strings.Contains(s.Identifier, "oomph:") {
		panic(oerror.New("malicious payload detected"))
	}

	cancel := false
	for _, h := range p.packetHandlers {
		cancel = cancel || !h.HandleClientPacket(pk, p)
		defer h.Defer()
	}

	if !p.RunDetections(pk) || cancel {
		return nil
	}

	if p.serverConn != nil {
		return p.SendPacketToServer(pk)
	}

	p.PacketQueue = append(p.PacketQueue, pk)
	return nil
}

func (p *Player) handleOneFromServer(pk packet.Packet) error {
	span := sentry.StartSpan(p.SentryTransaction.Context(), fmt.Sprintf("p.handleOneFromServer(%T)", pk))
	defer span.Finish()

	cancel := false
	for _, h := range p.packetHandlers {
		cancel = cancel || !h.HandleServerPacket(pk, p)
	}

	if cancel {
		return nil
	}

	return p.SendPacketToClient(pk)
}

// RegisterHandler registers a handler to the player.
func (p *Player) RegisterHandler(h Handler) {
	p.packetHandlers = append(p.packetHandlers, h)
}

// UnregisterHandler unregisters a handler from the player.
func (p *Player) UnregisterHandler(id string) {
	for i, h := range p.packetHandlers {
		if h.ID() != id {
			continue
		}

		p.packetHandlers = append(p.packetHandlers[:i], p.packetHandlers[i+1:]...)
		return
	}
}

// Handler returns a handler from the player.
func (p *Player) Handler(id string) Handler {
	for _, h := range p.packetHandlers {
		if h.ID() == id {
			return h
		}
	}

	return nil
}

// HandleEvents sets the event handler for the player.
func (p *Player) HandleEvents(h EventHandler) {
	p.eventHandler = h
}

// EventHandler returns the event handler for the player.
func (p *Player) EventHandler() EventHandler {
	return p.eventHandler
}

// SendRemoteEvent sends an Oomph event to the remote server.
// TODO: Better way to do this. Please.
func (p *Player) SendRemoteEvent(e RemoteEvent) {
	if p.serverConn == nil {
		return
	}

	enc, _ := json.Marshal(e)
	p.SendPacketToServer(&packet.ScriptMessage{
		Identifier: e.ID(),
		Data:       enc,
	})
}

// RegisterDetection registers a detection to the player.
func (p *Player) RegisterDetection(d Handler) {
	p.detections = append(p.detections, d)
}

// UnregisterDetection unregisters a detection from the player.
func (p *Player) UnregisterDetection(id string) {
	for i, d := range p.detections {
		if d.ID() != id {
			continue
		}

		p.detections = append(p.detections[:i], p.detections[i+1:]...)
		return
	}
}

// Detections returns all the detections registered to the player.
func (p *Player) Detections() []Handler {
	return p.detections
}

// RunDetections runs all the detections registered to the player. It returns false
// if the detection determines that the packet given should be dropped.
func (p *Player) RunDetections(pk packet.Packet) bool {
	cancel := false
	for _, d := range p.detections {
		span := sentry.StartSpan(p.SentryTransaction.Context(), fmt.Sprintf("%T.Run()", d))
		cancel = cancel || !d.HandleClientPacket(pk, p)
		span.Finish()
	}

	return !cancel
}

// Message sends a message to the player.
func (p *Player) Message(msg string, args ...interface{}) {
	p.SendPacketToClient(&packet.Text{
		TextType: packet.TextTypeChat,
		Message:  "§l§eoomph§7§r » " + text.Colourf(msg, args...),
	})
}

// Log returns the player's logger.
func (p *Player) Log() *logrus.Logger {
	return p.log
}

// SetLog sets the player's logger.
func (p *Player) SetLog(log *logrus.Logger) {
	p.log = log
}

// Disconnect disconnects the player with the given reason.
func (p *Player) Disconnect(reason string) {
	if p.MState.IsReplay {
		return
	}

	p.SendPacketToClient(&packet.Disconnect{
		Message: reason,
	})
	p.conn.Close()

	if p.serverConn != nil {
		p.serverConn.Close()
	}
}

// ReadBatchMode returns true if the player is configured to read batches of packets from the client.
func (p *Player) ReadBatchMode() bool {
	return p.readingBatches
}

// Close closes the player.
func (p *Player) Close() error {
	p.CloseFunc.Do(func() {
		p.Connected = false
		p.Closed = true

		p.ClientPkFunc = nil
		p.ServerPkFunc = nil

		p.eventHandler.HandleQuit(p)
		p.World.PurgeChunks()

		if !p.MState.IsReplay {
			p.conn.Close()
			if p.serverConn != nil {
				p.serverConn.Close()
			}
		}

		close(p.CloseChan)
	})

	return nil
}

// tick ticks handlers and checks, and also flushes connections. It returns false if the player should be removed.
func (p *Player) Tick() bool {
	p.SentryTransaction = sentry.StartTransaction(
		context.Background(),
		"oomph:tick",
		sentry.WithOpName("p.tick()"),
		sentry.WithDescription("Ticking the player"),
	)

	defer func() {
		p.SentryTransaction.Status = sentry.SpanStatusOK
		if time.Since(p.SentryTransaction.StartTime) >= targetedProcessingDelay {
			p.SentryTransaction.Status = sentry.SpanStatusDeadlineExceeded
		}

		p.SentryTransaction.Finish()
	}()

	p.ProcessMu.Lock()
	defer p.ProcessMu.Unlock()

	if p.Closed {
		return false
	}

	delta := time.Since(p.LastServerTick).Milliseconds()
	p.LastServerTick = p.Time()

	if delta >= 100 {
		p.ServerTick += (delta / 50) - 1
	} else {
		p.ServerTick++
	}

	// Tick all the handlers.
	for _, h := range p.packetHandlers {
		subSpan := sentry.StartSpan(p.SentryTransaction.Context(), fmt.Sprintf("%T.OnTick()", h))
		h.OnTick(p)
		subSpan.Finish()
	}

	// Tick all the detections.
	for _, d := range p.detections {
		d.OnTick(p)
	}

	// We don't need to flush here, because Oomph will flush the connection after every batch read.
	if p.readingBatches || p.MState.IsReplay {
		return true
	}

	// Flush all the packets for the client to receive.
	if p.conn == nil {
		p.log.Error("p.conn is nil - cannot tick")
		return false
	}

	if err := p.conn.Flush(); err != nil {
		p.log.Error("client connection is closed")
		return false
	}

	// serverConn will be nil if direct mode w/ Dragonfly is used.
	if p.serverConn == nil {
		return true
	}

	// Flush all the packets for the server to receive.
	if err := p.serverConn.Flush(); err != nil {
		p.log.Error("server connection is closed")
		p.Disconnect("Proxy unexpectedly lost connection to remote server.")
		return false
	}

	return true
}
