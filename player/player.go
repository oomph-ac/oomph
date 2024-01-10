package player

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

	// GameMode is the gamemode of the player. The player is exempt from movement predictions
	// if they are not in survival or adventure mode.
	GameMode int32

	// conn is the connection to the client, and serverConn is the connection to the server.
	conn, serverConn *minecraft.Conn
	// processMu is a mutex that is locked whenever packets need to be processed. It is used to
	// prevent race conditions, and to maintain accuracy with anti-cheat.
	// e.g - making sure all acknowledgements are sent in the same batch as the packets they are
	// being associated with.
	processMu sync.Mutex

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

		conn:       conn,
		serverConn: serverConn,

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

	go p.startTicking()
	return p
}

// Conn returns the connection to the client.
func (p *Player) Conn() *minecraft.Conn {
	return p.conn
}

// ServerConn returns the connection to the server.
func (p *Player) ServerConn() *minecraft.Conn {
	return p.serverConn
}

// HandleFromClient handles a packet from the client.
func (p *Player) HandleFromClient(pk packet.Packet) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	cancel := false
	for _, h := range p.packetHandlers {
		cancel = cancel || !h.HandleClientPacket(pk, p)
		defer h.Defer()
	}

	if !p.RunDetections(pk) || cancel {
		return nil
	}

	return p.serverConn.WritePacket(pk)
}

// HandleFromServer handles a packet from the server.
func (p *Player) HandleFromServer(pk packet.Packet) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()

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
func (p *Player) SendRemoteEvent(id string, data map[string]interface{}) {
	enc, _ := json.Marshal(data)
	p.serverConn.WritePacket(&packet.ScriptMessage{
		Identifier: id,
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

// Disconnect disconnects the player with the given reason.
func (p *Player) Disconnect(reason string) {
	p.conn.WritePacket(&packet.Disconnect{
		Message: reason,
	})
	p.conn.Flush()
}

// Close closes the player.
func (p *Player) Close() {
	p.once.Do(func() {
		p.Connected = false
		close(p.c)

		p.conn.Close()
		p.conn = nil

		if p.serverConn != nil {
			p.serverConn.Close()
			p.serverConn = nil
		}

		p.packetHandlers = nil
		p.detections = nil
	})
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
	if p.conn != nil {
		if err := p.conn.Flush(); err != nil {
			p.log.Errorf("error flushing packets to client: %v", err)
			return false
		}
	}

	// serverConn will be nil if direct mode w/ Dragonfly is used.
	if p.serverConn != nil {
		// Flush all the packets for the server to receive.
		if err := p.serverConn.Flush(); err != nil {
			p.log.Errorf("error flushing packets to server: %v", err)
			return false
		}
	}

	return true
}
