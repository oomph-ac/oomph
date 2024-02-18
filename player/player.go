package player

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

type Player struct {
	// Connected is true if the player is connected to Oomph.
	Connected bool
	Closed    bool

	Alive bool
	Ready bool

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
	LastServerTick          time.Time

	World *world.World

	// GameMode is the gamemode of the player. The player is exempt from movement predictions
	// if they are not in survival or adventure mode.
	GameMode int32

	// conn is the connection to the client, and serverConn is the connection to the server.
	conn, serverConn *minecraft.Conn
	// packetQueue is a queue of client packets that are to be processed by the server. This is only used
	// in direct mode.
	packetQueue []packet.Packet
	// processMu is a mutex that is locked whenever packets need to be processed. It is used to
	// prevent race conditions, and to maintain accuracy with anti-cheat.
	// e.g - making sure all acknowledgements are sent in the same batch as the packets they are
	// being associated with.
	processMu deadlock.Mutex

	// packetHandlers contains packet packetHandlers registered to the player.
	packetHandlers []Handler
	// detections contains packet handlers specifically used for detections.
	detections []Handler

	// eventHandler is a handler that handles events such as punishments and flags from detections.
	eventHandler EventHandler

	log *logrus.Logger

	c    chan bool
	once sync.Once
}

// New creates and returns a new Player instance.
func New(log *logrus.Logger, conn, serverConn *minecraft.Conn) *Player {
	p := &Player{
		Connected: true,

		CombatMode:   AuthorityModeSemi,
		MovementMode: AuthorityModeSemi,

		World: world.New(),

		conn:        conn,
		serverConn:  serverConn,
		packetQueue: []packet.Packet{},

		ClientTick:  0,
		ClientFrame: 0,
		ServerTick:  0,

		RuntimeId:       conn.GameData().EntityRuntimeID,
		ClientRuntimeId: conn.GameData().EntityRuntimeID,
		UniqueId:        conn.GameData().EntityUniqueID,
		ClientUniqueId:  conn.GameData().EntityUniqueID,
		IDModified:      false,

		packetHandlers: []Handler{},
		detections:     []Handler{},

		eventHandler: &NopEventHandler{},

		log: log,
		c:   make(chan bool),
	}

	if serverConn != nil {
		p.GameMode = serverConn.GameData().PlayerGameMode
		if p.GameMode == 5 {
			p.GameMode = serverConn.GameData().WorldGameMode
		}
	}

	go p.startTicking()
	return p
}

// HandleFromClient handles a packet from the client.
func (p *Player) HandleFromClient(pk packet.Packet) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	if p.Closed {
		return nil
	}

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
		return p.serverConn.WritePacket(pk)
	}

	p.packetQueue = append(p.packetQueue, pk)
	return nil
}

// HandleFromServer handles a packet from the server.
func (p *Player) HandleFromServer(pk packet.Packet) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	if p.Closed {
		return nil
	}

	cancel := false
	for _, h := range p.packetHandlers {
		cancel = cancel || !h.HandleServerPacket(pk, p)
	}

	if cancel {
		return nil
	}

	return p.conn.WritePacket(pk)
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
	p.serverConn.WritePacket(&packet.ScriptMessage{
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

// RunDetections runs all the detections registered to the player. It returns false
// if the detection determines that the packet given should be dropped.
func (p *Player) RunDetections(pk packet.Packet) bool {
	cancel := false
	for _, d := range p.detections {
		cancel = cancel || !d.HandleClientPacket(pk, p)
	}

	return !cancel
}

// Message sends a message to the player.
func (p *Player) Message(msg string) {
	p.conn.WritePacket(&packet.Text{
		TextType: packet.TextTypeChat,
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
	p.conn.WritePacket(&packet.Disconnect{
		Message: reason,
	})
	p.conn.Flush()

	p.conn.Close()
	if p.serverConn != nil {
		p.serverConn.Close()
	}
}

// Close closes the player.
func (p *Player) Close() error {
	p.once.Do(func() {
		p.Connected = false
		p.Closed = true

		p.eventHandler.HandleQuit(p)
		p.World.PurgeChunks()

		p.conn.Close()
		if p.serverConn != nil {
			p.serverConn.Close()
		}

		p.packetHandlers = nil
		p.eventHandler = nil
		p.detections = nil

		close(p.c)
	})

	return nil
}

// startTicking starts the player's tick loop.
func (p *Player) startTicking() {
	t := time.NewTicker(time.Millisecond * 50)
	defer t.Stop()

	for {
		select {
		case <-p.c:
			return
		case <-t.C:
			if !p.tick() {
				return
			}
		}
	}
}

// tick ticks handlers and checks, and also flushes connections. It returns false if the player should be removed.
func (p *Player) tick() bool {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	if p.Closed {
		return false
	}

	p.LastServerTick = time.Now()
	p.ServerTick++

	// Tick all the handlers.
	for _, h := range p.packetHandlers {
		h.OnTick(p)
	}

	// Tick all the detections.
	for _, d := range p.detections {
		d.OnTick(p)
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
