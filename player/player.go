package player

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/check"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

const DefaultNetworkLatencyCutoff int64 = 6

// Player contains information about a player, such as its virtual world or AABB.
type Player struct {
	log              *logrus.Logger
	conn, serverConn *minecraft.Conn

	debugger *Debugger

	rid uint64
	uid int64

	movementMode utils.AuthorityType
	combatMode   utils.AuthorityType

	locale language.Tag

	acks *Acknowledgements

	packetBuffer                                    *PacketBuffer
	bufferQueue                                     []*PacketBuffer
	expectedFrame, queueCredits                     uint64
	isBufferReady, isBufferStarted, usePacketBuffer bool
	excessBufferScore, depletedBufferScore          float64
	buffListMu                                      sync.Mutex

	clientTick, clientFrame, serverTick atomic.Uint64
	lastServerTicked                    time.Time

	hMutex sync.RWMutex
	h      Handler

	entity *entity.Entity

	entityMu sync.Mutex
	entities map[uint64]*entity.Entity

	effects  map[int32]effect.Effect
	effectMu sync.Mutex

	mInfo *MovementInfo
	miMu  sync.Mutex

	queueMu               sync.Mutex
	queuedEntityLocations map[uint64]utils.LocationData

	gameMode  int32
	inputMode uint32

	ready                 bool
	respawned             bool
	dead                  bool
	needsCombatValidation bool
	closed                bool

	isSyncedWithServer, awaitingSync bool
	nextSyncTick                     uint64

	inLoadedChunk      bool
	inLoadedChunkTicks uint64

	clickMu       sync.Mutex
	clicking      bool
	clicks        []uint64
	lastClickTick uint64
	clickDelay    uint64
	cps           int

	stackLatency      int64
	needLatencyUpdate bool

	knockbackNetworkCutoff int64
	combatNetworkCutoff    int64

	lastAttackData     *packet.InventoryTransaction
	lastEquipmentData  *packet.MobEquipment
	lastSentAttributes *packet.UpdateAttributes
	lastSentActorData  *packet.SetActorData

	nextTickActions   []func()
	nextTickActionsMu sync.Mutex

	chunks      map[protocol.ChunkPos]*chunk.Chunk
	chunkRadius int32
	chkMu       sync.Mutex

	checkMu sync.Mutex
	checks  []check.Check

	toSend []packet.Packet
	tMu    sync.Mutex

	c    chan struct{}
	once sync.Once

	world.NopViewer
}

// NewPlayer creates a new player from the given identity data, client data, position, and world.
func NewPlayer(log *logrus.Logger, conn, serverConn *minecraft.Conn) *Player {
	data := conn.GameData()
	p := &Player{
		log: log,

		packetBuffer: NewPacketBuffer(),
		bufferQueue:  make([]*PacketBuffer, 0),
		queueCredits: 0,

		debugger: &Debugger{},

		conn:       conn,
		serverConn: serverConn,

		rid: data.EntityRuntimeID,
		uid: data.EntityUniqueID,

		h: NopHandler{},

		acks: &Acknowledgements{
			AcknowledgeMap: make(map[int64][]func()),
		},

		entity: entity.NewEntity(
			data.PlayerPosition,
			mgl32.Vec3{},
			mgl32.Vec3{data.Pitch, data.Yaw, data.Yaw},
			true,
		),

		entities:              make(map[uint64]*entity.Entity),
		queuedEntityLocations: make(map[uint64]utils.LocationData),

		effects: make(map[int32]effect.Effect),

		stackLatency:      0,
		needLatencyUpdate: true,

		gameMode: data.PlayerGameMode,

		inLoadedChunk: false,

		c: make(chan struct{}),

		toSend: make([]packet.Packet, 0),

		checks: []check.Check{
			check.NewAutoClickerA(),
			check.NewAutoClickerB(),
			check.NewAutoClickerC(),
			check.NewAutoClickerD(),

			check.NewKillAuraA(),

			check.NewReachA(),
			check.NewReachB(),

			check.NewMovementA(),
			check.NewMovementB(),

			check.NewVelocityA(),
			check.NewVelocityB(),

			check.NewOSSpoofer(),

			check.NewTimerA(),

			check.NewInvalidA(),
		},

		mInfo: &MovementInfo{
			Speed: 0.1,
		},

		movementMode: utils.ModeFullAuthoritative,
		combatMode:   utils.ModeFullAuthoritative,

		knockbackNetworkCutoff: DefaultNetworkLatencyCutoff,
		combatNetworkCutoff:    DefaultNetworkLatencyCutoff,

		chunks: make(map[protocol.ChunkPos]*chunk.Chunk),

		nextTickActions: make([]func(), 0),
	}

	p.locale, _ = language.Parse(strings.Replace(conn.ClientData().LanguageCode, "_", "-", 1))
	p.chunkRadius = math.MaxInt16
	p.Acknowledgements().Refresh()

	go p.startTicking()
	return p
}

// SetRuntimeID sets the runtime ID of the player.
func (p *Player) SetRuntimeID(id uint64) {
	p.rid = id
}

// Conn returns the connection of the player.
func (p *Player) Conn() *minecraft.Conn {
	return p.conn
}

// Log returns the log of the player.
func (p *Player) Log() *logrus.Logger {
	return p.log
}

// Move moves the player to the given position.
func (p *Player) Move(pk *packet.PlayerAuthInput) {
	data, pos := p.Entity(), pk.Position
	data.Move(pos, true)
	data.Rotate(mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw})
	data.IncrementTeleportationTicks()

	p.mInfo.ClientMovement = p.Position().Sub(data.LastPosition())
	p.mInfo.ClientPredictedMovement = pk.Delta
}

// Teleport sets the position of the player and resets the teleport ticks of the player.
func (p *Player) Teleport(pos mgl32.Vec3) {
	pos = pos.Sub(mgl32.Vec3{0, 1.62})

	p.miMu.Lock()
	p.mInfo.Teleporting = true
	p.mInfo.CanExempt = true
	p.mInfo.ServerPosition = pos
	p.miMu.Unlock()
}

// MoveEntity moves an entity to the given position.
func (p *Player) MoveEntity(rid uint64, pos mgl32.Vec3, teleport bool, ground bool) {
	// If the entity exists, we can queue the location for an update.
	if e, ok := p.SearchEntity(rid); ok {
		e.SetServerPosition(pos)

		p.queueMu.Lock()
		p.queuedEntityLocations[rid] = utils.LocationData{
			Tick:     p.serverTick.Load(),
			Position: pos,
			OnGround: ground,
			Teleport: teleport,
		}
		p.queueMu.Unlock()
	}
}

// Locale returns the locale of the player.
func (p *Player) Locale() language.Tag {
	return p.locale
}

// Entity returns the entity data of the player.
func (p *Player) Entity() *entity.Entity {
	return p.entity
}

// ServerTick returns the current server tick.
func (p *Player) ServerTick() uint64 {
	return p.serverTick.Load()
}

// ServerTicked will return true if the ticking goroutine has ticked within the past 50 milliseconds.
func (p *Player) ServerTicked() bool {
	return time.Since(p.lastServerTicked).Milliseconds() <= 50
}

// ClientTick returns the current client tick. This is measured by the amount of PlayerAuthInput packets the
// client has sent. (since the packet is sent every client tick)
func (p *Player) ClientTick() uint64 {
	return p.clientTick.Load()
}

// ClientFrame returns the client's simulation frame given in the latest input packet
func (p *Player) ClientFrame() uint64 {
	return p.clientFrame.Load()
}

// Position returns the position of the player.
func (p *Player) Position() mgl32.Vec3 {
	return p.Entity().Position()
}

// Rotation returns the rotation of the player.
func (p *Player) Rotation() mgl32.Vec3 {
	return p.Entity().Rotation()
}

// AABB returns the axis-aligned bounding box of the player.
func (p *Player) AABB() cube.BBox {
	bb := p.Entity().AABB()
	pos := p.Position()
	if p.movementMode != utils.ModeClientAuthoritative {
		pos = p.mInfo.ServerPosition
	}

	return cube.Box(bb.Min().X(), bb.Min().Y(), bb.Min().Z(), bb.Max().X(), bb.Max().Y(), bb.Max().Z()).Translate(pos)
}

func (p *Player) Acknowledgements() *Acknowledgements {
	return p.acks
}

// MovementInfo returns the movement information of the player
func (p *Player) MovementInfo() *MovementInfo {
	p.miMu.Lock()
	defer p.miMu.Unlock()
	return p.mInfo
}

// TakingKnockback returns whether the player is currently taking knockback.
func (p *Player) TakingKnockback() bool {
	p.miMu.Lock()
	defer p.miMu.Unlock()
	return p.mInfo.MotionTicks <= 1
}

// UpdateServerVelocity updates the server velocity of the player.
func (p *Player) UpdateServerVelocity(v mgl32.Vec3) {
	p.miMu.Lock()
	p.mInfo.UpdateServerSentVelocity(v)
	p.miMu.Unlock()
}

// ClientMovement returns the client's movement as a Vec3
func (p *Player) ClientMovement() mgl32.Vec3 {
	p.miMu.Lock()
	defer p.miMu.Unlock()
	return p.mInfo.ClientMovement
}

// ServerMovement returns a Vec3 of how the server predicts the client will move.
func (p *Player) ServerMovement() mgl32.Vec3 {
	p.miMu.Lock()
	defer p.miMu.Unlock()
	return p.mInfo.ServerMovement
}

// OldServerMovement returns a Vec3 of how the server predicted the client moved in the previous tick.
func (p *Player) OldServerMovement() mgl32.Vec3 {
	p.miMu.Lock()
	defer p.miMu.Unlock()
	return p.mInfo.OldServerMovement
}

// MovementMode returns the movement mode of the player. The player's movement mode will determine how
// much authority over movement oomph has.
func (p *Player) MovementMode() utils.AuthorityType {
	return p.movementMode
}

// CombatMode returns the combat mode of the player. The combat mode will determine how much authority
// over combat oomph has.
func (p *Player) CombatMode() utils.AuthorityType {
	return p.combatMode
}

// SetMovementMode sets the movement authority for the player.
func (p *Player) SetMovementMode(mode utils.AuthorityType) {
	p.movementMode = mode
}

// SetCombatMode sets the combat authority for the player.
func (p *Player) SetCombatMode(mode utils.AuthorityType) {
	p.combatMode = mode
}

// SetKnockbackCutoff sets the amount of ticks of latency allowed before using server-authoritative knockback.
// This will only have an affect on server-authoritative movement.
func (p *Player) SetKnockbackCutoff(i int64) {
	p.knockbackNetworkCutoff = i
}

// SetCombatCutoff sets the max amount of ticks allowed of rewind used for server-authoritative combat. This will not affect client
// authoritative combat and semi-authoritative combat.
func (p *Player) SetCombatCutoff(i int64) {
	p.combatNetworkCutoff = i
}

// Acknowledgement runs a function after an acknowledgement from the client.
// TODO: Find something with similar usage to NSL - it will possibly be removed in future versions of Minecraft
func (p *Player) Acknowledgement(f func()) {
	// Do not attempt to send an acknowledgement if the player is closed
	if p.closed {
		return
	}

	p.Acknowledgements().Add(f)
}

func (p *Player) OnNextClientTick(f func()) {
	p.nextTickActionsMu.Lock()
	defer p.nextTickActionsMu.Unlock()

	p.nextTickActions = append(p.nextTickActions, f)
}

// Debug debugs the given check data to the console and other relevant sources.
func (p *Player) Debug(check check.Check, params map[string]any) {
	name, variant := check.Name()
	ctx := event.C()
	p.handler().HandleDebug(ctx, check, params)
	if !ctx.Cancelled() {
		p.log.Debugf("%s (%s%s): %s", p.Name(), name, variant, utils.PrettyParameters(params, true))
	}
}

// Flag flags the given check data to the console and other relevant sources.
func (p *Player) Flag(check check.Check, violations float64, params map[string]any) {
	if violations <= 0 {
		// No violations, don't flag anything.
		return
	}

	name, variant := check.Name()
	check.AddViolation(violations)

	params["latency"] = fmt.Sprint(p.stackLatency, "ms")

	ctx := event.C()
	log := true
	p.handler().HandleFlag(ctx, check, params, &log)
	if ctx.Cancelled() {
		return
	}

	if log {
		p.log.Infof("%s was flagged for %s%s: %s", p.Name(), name, variant, utils.PrettyParameters(params, true))
	}

	if now, max := check.Violations(), check.MaxViolations(); now < max {
		return
	}

	go func() {
		message := fmt.Sprintf("§7[§6oomph§7] §bcaught lackin!\n§6cheat detected: §b%s%s", name, variant)

		ctx = event.C()
		p.handler().HandlePunishment(ctx, check, &message)
		if !ctx.Cancelled() {
			p.log.Infof("%s was detected and punished for using %s%s.", p.Name(), name, variant)
			p.Disconnect(message)
		}
	}()
}

// Ready returns true if the player is ready/spawned in.
func (p *Player) Ready() bool {
	return p.ready
}

// IsSyncedWithServer returns true if the player has responded to an acknowledgement when
// attempting to sync client and server ticks.
func (p *Player) IsSyncedWithServer() bool {
	return p.isSyncedWithServer
}

// GameMode returns the current game mode of the player.
func (p *Player) GameMode() int32 {
	return p.gameMode
}

// InputMode returns the input mode of the player
func (p *Player) InputMode() uint32 {
	return p.inputMode
}

// Sneaking returns true if the player is currently sneaking.
func (p *Player) Sneaking() bool {
	return p.MovementInfo().Sneaking
}

// Sprinting returns true if the player is currently sprinting.
func (p *Player) Sprinting() bool {
	return p.MovementInfo().Sprinting
}

// OnGround returns true if the player is currently on the ground.
func (p *Player) OnGround() bool {
	return p.MovementInfo().OnGround
}

// Teleporting returns true if the player is currently teleporting.
func (p *Player) Teleporting() bool {
	return p.MovementInfo().Teleporting
}

// Jumping returns true if the player is currently jumping.
func (p *Player) Jumping() bool {
	return p.MovementInfo().Jumping
}

// Immobile returns true if the player is currently immobile.
func (p *Player) Immobile() bool {
	return p.MovementInfo().Immobile
}

// Flying returns true if the player is currently flying.
func (p *Player) Flying() bool {
	return p.MovementInfo().Flying
}

// Dead returns true if the player is currently dead.
func (p *Player) Dead() bool {
	return p.dead
}

// Respawned returns true if the player is respawning into the world.
func (p *Player) Respawned() bool {
	return p.respawned
}

// SetRespawned sets the respawned state of the player.
func (p *Player) SetRespawned(v bool) {
	p.respawned = v
}

// InLoadedChunk returns true if the player is in a chunk loaded by its world
func (p *Player) InLoadedChunk() bool {
	return p.inLoadedChunk
}

// Clicking returns true if the player is clicking.
func (p *Player) Clicking() bool {
	return p.clicking
}

// Click adds a click to the player's click history.
func (p *Player) Click() {
	currentTick := p.ClientTick()

	p.clickMu.Lock()
	p.clicking = true
	if len(p.clicks) > 0 {
		p.clickDelay = (currentTick - p.lastClickTick) * 50
	} else {
		p.clickDelay = 0
	}
	p.clicks = append(p.clicks, currentTick)
	var clicks []uint64
	for _, clickTick := range p.clicks {
		if currentTick-clickTick <= 20 {
			clicks = append(clicks, clickTick)
		}
	}
	p.lastClickTick = currentTick
	p.clicks = clicks
	p.cps = len(p.clicks)
	p.clickMu.Unlock()
}

// CPS returns the clicks per second of the player.
func (p *Player) CPS() int {
	return p.cps
}

// ClickDelay returns the delay between the current click and the last one.
func (p *Player) ClickDelay() uint64 {
	return p.clickDelay
}

// StackLatency returns the tick stack latency of the player in milliseconds. This value also contains
// some processing delays of the server.
func (p *Player) StackLatency() int64 {
	return p.stackLatency
}

// TickLatency returns the tick stack latency of the player in ticks.
func (p *Player) TickLatency() int64 {
	return int64(p.ServerTick()) - int64(p.ClientTick())
}

// Name returns the player's display name.
func (p *Player) Name() string {
	return p.IdentityData().DisplayName
}

// SendOomphDebug sends a debug message to the processor.
func (p *Player) SendOomphDebug(message string, t byte) {
	p.conn.WritePacket(&packet.Text{
		TextType: t,
		Message:  "§l§7[§eoomph§7]§r§f " + message,
		XUID:     "",
	})
}

// Disconnect disconnects the player for the reason provided.
func (p *Player) Disconnect(reason string) {
	_ = p.conn.WritePacket(&packet.Disconnect{Message: reason})
	p.Close()
}

// Closed returns if the player is closed.
func (p *Player) Closed() bool {
	return p.closed
}

// Close closes the player.
func (p *Player) Close() error {
	p.once.Do(func() {
		p.closed = true

		p.checkMu.Lock()
		p.checks = nil
		p.checkMu.Unlock()

		p.clearAllChunks()

		p.entityMu.Lock()
		p.entities = nil
		p.entityMu.Unlock()

		close(p.c)

		_ = p.conn.Close()
		if p.serverConn != nil {
			_ = p.serverConn.Close()
		}
	})

	return nil
}

// Handle sets the handler of the player.
func (p *Player) Handle(h Handler) {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()
	p.h = h
}

func (p *Player) flushConns() {
	p.sendAck()

	p.conn.Flush()
	if p.serverConn != nil {
		p.serverConn.Flush()
	}
}

// tickEntitiesPos ticks the position of all entities
func (p *Player) tickEntitiesPos() {
	p.entityMu.Lock()
	for _, e := range p.entities {
		e.TickPosition(p.serverTick.Load())
	}
	p.entityMu.Unlock()
}

// ackEntitiesPos prepares to acknowledge the position of all entities in the queue.
func (p *Player) ackEntitiesPos() {
	defer func() {
		p.queuedEntityLocations = make(map[uint64]utils.LocationData)
		p.queueMu.Unlock()
	}()
	p.queueMu.Lock()

	queue := p.queuedEntityLocations

	// If there's a position for the entity sent by the server, we will update the server position
	// of the entity to the position sent. This position is not one of the rewinded positions, but
	// rather the position sent by the server that will be interpolated later by e.TickPosition().
	if p.combatMode == utils.ModeFullAuthoritative {
		p.updateEntityPositions(queue)
		return
	}

	p.Acknowledgement(func() {
		p.updateEntityPositions(queue)
	})
}

// updateEntityPositions updates the positions of all entities in the queue passed in the function.
func (p *Player) updateEntityPositions(m map[uint64]utils.LocationData) {
	for rid, dat := range m {
		if e, valid := p.SearchEntity(rid); valid {
			e.UpdatePosition(dat, e.Player())
		}
	}
}

// updateLatency updates the stack latency of the player.
func (p *Player) updateLatency() {
	if !p.needLatencyUpdate || (p.usePacketBuffer && !p.isBufferStarted) {
		return
	}

	p.needLatencyUpdate = false
	curr := time.Now()

	p.Acknowledgement(func() {
		p.needLatencyUpdate = true
		p.stackLatency = time.Since(curr).Milliseconds()

		if !p.debugger.Latency {
			return
		}

		p.SendOomphDebug(fmt.Sprint("RTT + Processing Delays: ", p.stackLatency, "ms"), packet.TextTypePopup)
	})
}

func (p *Player) tryDoSync() {
	sTick := p.serverTick.Load()
	if sTick >= p.nextSyncTick {
		p.isSyncedWithServer = false
	}

	// This will sync the server's and client's tick to match. This is mainly
	// done for scenarios where the client will be simulating slower than the server.
	if !p.dead && !p.isSyncedWithServer && !p.awaitingSync {
		p.awaitingSync = true
		p.Acknowledgement(func() {
			p.clientTick.Store(sTick)
			p.isSyncedWithServer = true
			p.awaitingSync = false
			p.nextSyncTick = p.ServerTick() + uint64(p.TickLatency()) + 1
		})
	}
}

// startTicking ticks the player until the connection is closed.
func (p *Player) startTicking() {
	t := time.NewTicker(time.Second / 20)
	defer t.Stop()

	p.lastServerTicked = time.Now()
	for {
		select {
		case <-p.c:
			return
		case <-t.C:
			p.doTick()
		}
	}
}

// doTick ticks the player.
func (p *Player) doTick() {
	// This code calculates how much the server tick should be incremented by. This is done by checking the
	// difference between the last time the server ticked and the current time.
	delta := time.Since(p.lastServerTicked).Milliseconds()
	if delta > 50 {
		p.serverTick.Add(uint64(delta / 50))
	} else {
		p.serverTick.Inc()
	}

	// This will handle all the client packets if packet buffering is enabled.
	p.handlePacketQueue()

	// This will prepare the entity positions to be acknowledged.
	p.ackEntitiesPos()

	// This code ticks positions for entities on server tick, this is used for the rewind combat system, so that we
	// can rewind back in time to what the client sees. Of course, this is not 100% accurate to what the client sees due
	// to extra interpolation code in the client, but it should be close enough.
	if p.combatMode == utils.ModeFullAuthoritative {
		p.tickEntitiesPos()
	}

	// If the player is not responding to acknowledgements, we have to kick them to prevent
	// abusive behavior (bypasses).
	if !p.Acknowledgements().Validate() {
		p.Disconnect("Error: Client was unable to respond to acknowledgements sent by the server.")
	}

	p.tryDoSync()
	p.updateLatency()
	p.flushConns()

	p.lastServerTicked = time.Now()
}

// handler returns the handler of the player.
func (p *Player) handler() Handler {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()
	return p.h
}
