package player

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/utils"
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
	GameVersion1_20_50 = 630
	GameVersion1_20_60 = 649
	GameVersion1_20_70 = 662
	GameVersion1_20_80 = 671

	GameVersion1_21_0  = 685
	GameVersion1_21_2  = 686
	GameVersion1_21_20 = 712
	GameVersion1_21_30 = 729

	TicksPerSecond = 20

	targetedProcessingDelay = 10 * time.Millisecond
)

type Player struct {
	MState MonitoringState

	// Connected is true if the player is connected to Oomph.
	Connected bool
	Closed    bool

	Alive      bool
	TicksAlive int64

	Ready bool

	CloseChan chan bool
	CloseFunc sync.Once

	RunChan chan func()

	ClientPkFunc func([]packet.Packet) error
	ServerPkFunc func([]packet.Packet) error

	ClientDat   login.ClientData
	IdentityDat login.IdentityData
	GameDat     minecraft.GameData
	Version     int32

	// combatMode and movementMode are the authority modes of the player. They are used to determine
	// how certain actions should be handled.
	CombatMode AuthorityMode

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
	StackLatency            time.Duration
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

	// listener is the Gophertunnel listener
	listener *minecraft.Listener

	// PacketQueue is a queue of client packets that are to be processed by the server. This is only used
	// in direct mode.
	PacketQueue []packet.Packet
	// ProcessMu is a mutex that is locked whenever packets need to be processed. It is used to
	// prevent race conditions, and to maintain accuracy with anti-cheat.
	// e.g - making sure all acknowledgements are sent in the same batch as the packets they are
	// being associated with.
	ProcessMu deadlock.Mutex

	// Dbg is the debugger of the player. It is used to log debug messages to the player.
	Dbg *Debugger

	acks           AcknowledgmentComponent
	effects        EffectsComponent
	entTracker     EntityTrackerComponent
	gamemodeHandle GamemodeComponent
	movement       MovementComponent
	worldUpdater   WorldUpdaterComponent // TODO: figure out a name for this shit.

	// packetHandlers contains packet packetHandlers registered to the player.
	packetHandlers []Handler
	// detections contains packet handlers specifically used for detections.
	detections []Handler

	// eventHandler is a handler that handles events such as punishments and flags from detections.
	eventHandler EventHandler

	// log is the logger of the player.
	log *logrus.Logger
}

// New creates and returns a new Player instance.
func New(log *logrus.Logger, mState MonitoringState, listener *minecraft.Listener) *Player {
	p := &Player{
		MState: mState,

		Connected: true,

		CombatMode: AuthorityModeSemi,

		World: world.New(),

		PacketQueue: []packet.Packet{},

		ClientTick:     0,
		ClientFrame:    0,
		ServerTick:     0,
		LastServerTick: time.Now(),
		Tps:            20.0,

		CloseChan: make(chan bool),
		RunChan:   make(chan func(), 32),

		packetHandlers: []Handler{},
		detections:     []Handler{},

		eventHandler: &NopEventHandler{},

		log: log,

		listener: listener,
	}

	p.Dbg = NewDebugger(p)
	p.ClientPkFunc = p.DefaultHandleFromClient
	p.ServerPkFunc = p.DefaultHandleFromServer

	go p.startTicking()
	return p
}

// RunWhenFree runs a function when the player is free to do so.
func (p *Player) RunWhenFree(f func()) {
	p.RunChan <- f
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
		p.PacketQueue = append(p.PacketQueue, pk)
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

// RegisterHandler registers a handler to the player.
func (p *Player) RegisterHandler(h Handler) {
	p.packetHandlers = append(p.packetHandlers, h)
}

// ReplaceHandler replaces a handler in the player.
func (p *Player) ReplaceHandler(id string, h Handler) {
	for i, otherH := range p.packetHandlers {
		if otherH.ID() != id {
			continue
		}

		p.packetHandlers[i] = nil
		p.packetHandlers[i] = h
		return
	}

	panic(oerror.New("handler %s not found", id))
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

// NMessage sends a message to the player without the oomph prefix.
func (p *Player) NMessage(msg string, args ...interface{}) {
	p.SendPacketToClient(&packet.Text{
		TextType: packet.TextTypeChat,
		Message:  text.Colourf(msg, args...),
	})
}

func (p *Player) Popup(msg string) {
	p.SendPacketToClient(&packet.Text{
		TextType: packet.TextTypeJukeboxPopup,
		Message:  msg,
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

func (p *Player) BlockAddress(duration time.Duration) {
	if rkListener, ok := utils.GetRaknetListener(p.listener); ok {
		utils.BlockAddress(rkListener, p.RemoteAddr().(*net.UDPAddr).IP, duration)
	}
}

// Close closes the player.
func (p *Player) Close() error {
	p.CloseFunc.Do(func() {

		p.Connected = false
		p.Closed = true

		p.eventHandler.HandleQuit(p)
		p.World.PurgeChunks()

		if !p.MState.IsReplay {
			p.conn.Close()
			if p.serverConn != nil {
				p.serverConn.Close()
			}
		}

		p.Dbg.target = nil
		close(p.CloseChan)
	})

	return nil
}

func (p *Player) startTicking() {
	t := time.NewTicker(time.Millisecond * 50)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if !p.tick() {
				return
			}
		case <-p.CloseChan:
			return
		}
	}
}

// tick ticks handlers and checks, and also flushes connections. It returns false if the player should be removed.
func (p *Player) tick() bool {
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

	p.EntityTracker().Tick(p.ServerTick)
	p.ACKs().Tick()
	p.ACKs().Flush()

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

	if err := p.conn.Flush(); err != nil {
		return false
	}
	return true
}

// calculateBBSize calculates the bounding box size for an entity based on the EntityMetadata.
func calculateBBSize(data map[uint32]any, defaultWidth, defaultHeight, defaultScale float32) (width float32, height float32, s float32) {
	width = defaultWidth
	if w, ok := data[entity.DataKeyBoundingBoxWidth]; ok {
		width = w.(float32)
	}

	height = defaultHeight
	if h, ok := data[entity.DataKeyBoundingBoxHeight]; ok {
		height = h.(float32)
	}

	s = defaultScale
	if scale, ok := data[entity.DataKeyScale]; ok {
		s = scale.(float32)
	}

	return
}
