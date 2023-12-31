package player

import (
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type Player struct {
	// conn is the connection to the client, and serverConn is the connection to the server.
	conn, serverConn *minecraft.Conn
	// processMu is a mutex that is locked whenever packets need to be processed. It is used to
	// prevent race conditions, and to maintain accuracy with anti-cheat.
	// e.g - making sure all acknowledgements are sent in the same batch as the packets they are
	// being associated with.
	processMu sync.Mutex

	// With fast transfers, the client will still retain it's original runtime and unique IDs, so
	// we must translate them to new ones, while still retaining the old ones for the client to use.
	runtimeId, clientRuntimeId int64
	uniqueId, clientUniqueId   int64

	// clientTick is the tick of the client, synchronized with the server's on an interval.
	// clientFrame is the simulation frame of the client, sent in PlayerAuthInput.
	clientTick, clientFrame int64
	serverTick              int64

	// handlers contains packet handlers registered to the player.
	handlers []Handler
	// detections contains packet handlers specifically used for detections.
	detections []Handler

	log *logrus.Logger

	c    chan bool
	once sync.Once
}

// New creates and returns a new Player instance.
func New(log *logrus.Logger, conn, serverConn *minecraft.Conn) *Player {
	p := &Player{
		conn:       conn,
		serverConn: serverConn,

		clientTick:  0,
		clientFrame: 0,
		serverTick:  0,

		runtimeId:       -1,
		clientRuntimeId: -1,
		uniqueId:        -1,
		clientUniqueId:  -1,

		handlers:   []Handler{},
		detections: []Handler{},

		log: log,
		c:   make(chan bool),
	}

	// NOTE: Handlers must be registered in order in terms of priority.
	p.RegisterHandler(&LatencyHandler{})
	p.RegisterHandler(&AcknowledgementHandler{
		AckMap: map[int64][]func(){},
	})
	p.RegisterHandler(&MovementHandler{})

	go p.startTicking()
	return p
}

// HandleFromClient handles a packet from the client.
func (p *Player) HandleFromClient(pk packet.Packet) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	cancel := false
	for _, h := range p.handlers {
		cancel = cancel || !h.HandleClientPacket(pk, p)
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
	for _, h := range p.handlers {
		cancel = cancel || !h.HandleServerPacket(pk, p)
	}

	if cancel {
		return nil
	}

	return p.conn.WritePacket(pk)
}

// RegisterHandler registers a handler to the player.
func (p *Player) RegisterHandler(h Handler) {
	p.handlers = append(p.handlers, h)
}

// UnregisterHandler unregisters a handler from the player.
func (p *Player) UnregisterHandler(id string) {
	for i, h := range p.handlers {
		if h.ID() != id {
			continue
		}

		p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
		return
	}
}

// Handler returns a handler from the player.
func (p *Player) Handler(id string) Handler {
	for _, h := range p.handlers {
		if h.ID() == id {
			return h
		}
	}

	return nil
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

func (p *Player) Close() {
	p.once.Do(func() {
		close(p.c)

		p.conn = nil
		p.serverConn = nil

		p.handlers = nil
		p.detections = nil
	})
}

func (p *Player) startTicking() {
	t := time.NewTicker(time.Millisecond)
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

// tick returns false if the player should be removed.
func (p *Player) tick() bool {
	p.processMu.Lock()
	defer p.processMu.Unlock()

	p.serverTick++

	// Tick all the handlers.
	for _, h := range p.handlers {
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
