package player

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/event"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/utils"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
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
	GameVersion1_21_40 = 748
	GameVersion1_21_50 = 766
	GameVersion1_21_60 = 776
	GameVersion1_21_70 = 786
	GameVersion1_21_80 = protocol.CurrentProtocol

	TicksPerSecond = 20
)

type Player struct {
	MState MonitoringState

	Ready  bool
	Closed bool

	Alive      bool
	TicksAlive int64

	CloseChan chan bool
	CloseFunc sync.Once

	RunChan chan func()

	ClientDat   login.ClientData
	IdentityDat login.IdentityData
	GameDat     minecraft.GameData
	Version     int32

	// With fast transfers, the client will still retain it's original runtime and unique IDs, so
	// we must translate them to new ones, while still retaining the old ones for the client to use.
	RuntimeId uint64
	UniqueId  int64

	// ClientTick is the tick of the client, synchronized with the server's on an interval.
	// InputCount is the amount of PlayerAuthInput packets the client has sent.
	// SimulationFrame is the simulation frame of the client, sent in PlayerAuthInput.
	SimulationFrame        uint64
	ClientTick, InputCount int64
	ServerTick             int64
	Tps                    float32
	StackLatency           time.Duration
	LastServerTick         time.Time

	// PendingCorrectionACK is a boolean indicating whether the client has received a correction
	// Oomph has sent to it. This is used to not spam the client with mutliple corrections which may
	// cause desync due to the client's own interpolation.
	PendingCorrectionACK bool

	// GameMode is the gamemode of the player. The player is exempt from movement predictions
	// if they are not in survival or adventure mode.
	GameMode int32
	// InputMode is the input mode of the player.
	InputMode uint32

	// LastEquipmentData stores the last MobEquipment packet sent by the client to properly
	// make an attack packet when Oomph's full authoritative combat detects a misprediction.
	LastEquipmentData *packet.MobEquipment
	// LastActorMetadata is the last actor metadata packet sent by the client
	LastSetActorData *packet.SetActorData

	// blockBreakProgress (usually between 0 and 1) is how far along the player is from breaking a targeted block.
	blockBreakProgress float32
	// blockBreakStartTick is the tick that the player started breaking a block.
	blockBreakStartTick int64
	// blockBreakInProgress is a boolean indicating whether the player is currently breaking a block. This is used for
	// when server-authoritative block breaking is enabled on the server.
	blockBreakInProgress bool

	// lastUseProjectileTick is the last tick the player used a projectile item.
	lastUseProjectileTick int64
	// startUseConsumableTick is the tick that the player started using a consumable item.
	startUseConsumableTick int64
	// consumedSlot is the slot of the item that the player started consuming.
	consumedSlot int

	// conn is the connection to the client, and serverConn is the connection to the server.
	conn       *minecraft.Conn
	serverConn ServerConn

	world *world.World

	// listener is the Gophertunnel listener
	listener *minecraft.Listener

	// procMu is a mutex that is locked whenever packets need to be processed. It is used to
	// prevent race conditions, and to maintain accuracy with anti-cheat.
	// e.g - making sure all acknowledgements are sent in the same batch as the packets they are
	// being associated with.
	procMu sync.Mutex

	// Dbg is the debugger of the player. It is used to log debug messages to the player.
	Dbg *Debugger

	// deferredPackets is a slice of packets that is used when we are unable to write a
	// packet to the destination server.
	deferredPackets []packet.Packet

	// acks is the component that handles acknowledgments from the player.
	acks AcknowledgmentComponent
	// effects is the component that handles effect from the server sent to the player.
	effects EffectsComponent
	// gamemodeHandle is a component that sends acknowledgments whenever the server updates the player's gamemode.
	gamemodeHandle GamemodeComponent
	// movement is a component that handles updating movement states for the player and validating their movement.
	movement MovementComponent
	// worldUpdater is a component that handles chunk and block updates in the world for the player
	worldUpdater WorldUpdaterComponent // TODO: figure out a better name for this [sugar honey iced tea].
	// inventory is a component that handles inventory-related actions.
	inventory InventoryComponent

	// entTracker is an entity tracker for server-sided view of entities. It does not rely on the client sending back
	// acknowledgments to update positions and states of the entity.
	entTracker EntityTrackerComponent
	// clientEntTracker is an entity tracker for fully client-sided view of entities. It does rely on the client sending
	// back acknowledgments to update the position and state of the entity, which can allow exploits like backtrack to function.
	// However, this component is useful when presice information is needed - such as heuristics and enabling the reach detection
	// to flag players instead of just mitigating them.
	clientEntTracker EntityTrackerComponent

	// combat is the component that handles validating combat. This combat component does not rely on client ACKs to determine the position
	// and state of the entity. This component determines whether an attack should be sent to the server.
	combat CombatComponent
	// clientCombat is the component that handles validating combat with the client-state entity tracker.
	clientCombat CombatComponent

	// eventHandler is a handler that handles events such as punishments and flags from detections.
	eventHandler EventHandler

	// detections contains the detections for the player.
	detections []Detection

	// log is the logger of the player.
	log *slog.Logger

	recoverFunc func(p *Player, err any)
	closer      func()

	pkCtx *context.HandlePacketContext

	// remoteEventFunc is the function for sending remote events to the server
	remoteEventFunc func(e RemoteEvent, p *Player)

	opts *Opts

	df_world.NopViewer
}

// New creates and returns a new Player instance.
func New(log *slog.Logger, mState MonitoringState, listener *minecraft.Listener) *Player {
	p := &Player{
		MState: mState,

		ClientTick:      0,
		SimulationFrame: 0,
		ServerTick:      0,
		LastServerTick:  time.Now(),
		Tps:             20.0,

		CloseChan: make(chan bool),
		RunChan:   make(chan func(), 32),

		deferredPackets: make([]packet.Packet, 0, 256),

		detections: []Detection{},

		eventHandler: &NopEventHandler{},

		log: log,

		listener: listener,

		remoteEventFunc: func(e RemoteEvent, p *Player) {
			enc, _ := json.Marshal(e)
			p.SendPacketToServer(&packet.ScriptMessage{
				Identifier: e.ID(),
				Data:       enc,
			})
		},
	}
	if mState.IsReplay {
		p.LastServerTick = mState.CurrentTime
	}

	p.opts = new(Opts)
	p.opts.Combat = oconfig.Combat()
	p.opts.Movement = oconfig.Movement()
	p.opts.UseDebugCommands = oconfig.Cfg.UseDebugCommands

	p.world = world.New(func(msg string, args ...any) {
		p.Dbg.Notify(DebugModeChunks, true, msg, args...)
	})
	p.Dbg = NewDebugger(p)
	return p
}

func (p *Player) SetRecoverFunc(f func(p *Player, err any)) {
	p.recoverFunc = f
}

func (p *Player) SetRemoteEventFunc(f func(e RemoteEvent, p *Player)) {
	p.remoteEventFunc = f
}

func (p *Player) WithPacketCtx(f func(*context.HandlePacketContext)) {
	if p.pkCtx == nil {
		f(p.pkCtx)
	}
}

// PauseProcessing locks the procMu to prevent any packets from being processed.
func (p *Player) PauseProcessing() {
	p.procMu.Lock()
}

// ResumeProcessing unlocks the procMu to allow packets to be processed.
func (p *Player) ResumeProcessing() {
	p.procMu.Unlock()
}

// Name returns the name of the player.
func (p *Player) Name() string {
	return p.IdentityDat.DisplayName
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
	if pk == nil {
		return nil
	}

	if p.MState.IsReplay {
		return nil
	} else if p.serverConn == nil {
		p.deferredPackets = append(p.deferredPackets, pk)
		return nil
	}

	for _, dPk := range p.deferredPackets {
		if err := p.serverConn.WritePacket(dPk); err != nil {
			return err
		}
	}
	p.deferredPackets = p.deferredPackets[:0]

	return p.serverConn.WritePacket(pk)
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
func (p *Player) SendRemoteEvent(e RemoteEvent) {
	if p.serverConn == nil {
		return
	}

	p.remoteEventFunc(e, p)
}

// RegisterDetection registers a detection to the player.
func (p *Player) RegisterDetection(d Detection) {
	p.detections = append(p.detections, d)
}

// Detections returns all the detections registered to the player.
func (p *Player) Detections() []Detection {
	return p.detections
}

// RunDetections runs all the detections registered to the player. It returns false
// if the detection determines that the packet given should be dropped.
func (p *Player) RunDetections(pk packet.Packet) {
	for _, d := range p.detections {
		d.Detect(pk)
	}
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

func (p *Player) Popup(msg string, args ...interface{}) {
	p.SendPacketToClient(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text.Colourf(msg, args...),
	})
}

// Log returns the player's logger.
func (p *Player) Log() *slog.Logger {
	return p.log
}

// SetLog sets the player's logger.
func (p *Player) SetLog(log *slog.Logger) {
	p.log = log
}

// Disconnect disconnects the player with the given reason.
func (p *Player) Disconnect(reason string) {
	if p.MState.IsReplay {
		panic(fmt.Errorf("replay terminated: %v", reason))
	}
	p.SendPacketToClient(&packet.Disconnect{
		Message: reason,
	})
	p.Close()
}

func (p *Player) BlockAddress(duration time.Duration) {
	if rkListener, ok := utils.GetRaknetListener(p.listener); ok {
		utils.BlockAddress(rkListener, p.RemoteAddr().(*net.UDPAddr).IP, duration)
	}
}

func (p *Player) IsVersion(ver int32) bool {
	return p.Version == ver
}

func (p *Player) VersionInRange(oldest, latest int32) bool {
	return p.Version >= oldest && p.Version <= latest
}

func (p *Player) SetCloser(closer func()) {
	// If the player is already closed, we should not set the closer.
	if p.Closed {
		return
	}
	p.closer = closer
}

// Close closes the player.
func (p *Player) Close() error {
	p.CloseFunc.Do(func() {
		p.Closed = true
		if evHandler := p.eventHandler; evHandler != nil {
			evHandler.HandleQuit()
		}

		if !p.MState.IsReplay {
			if c := p.conn; c != nil {
				c.Close()
			}
			if c := p.serverConn; c != nil {
				c.Close()
			}
		}

		p.log = nil
		if conn := p.conn; conn != nil {
			p.conn.Close()
			p.conn = nil
		}
		if serverConn := p.serverConn; serverConn != nil {
			serverConn.Close()
			p.serverConn = nil
		}
		p.Dbg.target = nil
		p.world.PurgeChunks()
		close(p.CloseChan)

		if closer := p.closer; closer != nil {
			closer()
		}
	})

	return nil
}

func (p *Player) StartTicking() {
	t := time.NewTicker(time.Millisecond * 50)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if !p.Tick() {
				return
			}
		case <-p.CloseChan:
			return
		}
	}
}

// Tick ticks handlers and checks, and also flushes connections. It returns false if the player should be removed.
func (p *Player) Tick() bool {
	p.procMu.Lock()
	defer p.procMu.Unlock()

	if p.Closed {
		return false
	}

	delta := p.Time().Sub(p.LastServerTick).Milliseconds()
	p.LastServerTick = p.Time()

	prevTick := p.ServerTick
	if delta > 50 {
		p.ServerTick += (delta / 50) + 1
	} else {
		p.ServerTick++
	}

	p.movement.Tick(p.ServerTick - prevTick)
	p.entTracker.Tick(p.ServerTick)
	p.acks.Tick(false)
	if !p.acks.Responsive() {
		p.Disconnect(game.ErrorNetworkTimeout)
		return false
	}
	// If the player state is from a replay, we should only flush the acknowledgment component when
	// requested by the program handling the replay.
	if !p.MState.IsReplay {
		p.acks.Flush()
	}

	if h := p.eventHandler; h != nil {
		h.HandleTick(event.C(p))
	}

	if !p.MState.IsReplay {
		if err := p.conn.Flush(); err != nil {
			return false
		}
		if srvConn, ok := p.serverConn.(*minecraft.Conn); ok {
			if err := srvConn.Flush(); err != nil {
				return false
			}
		}
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
