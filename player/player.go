package player

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/check"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
	"golang.org/x/text/language"
)

const DefaultNetworkLatencyCutoff int64 = 6

// Player contains information about a player, such as its virtual world or AABB.
type Player struct {
	log              *logrus.Logger
	conn, serverConn *minecraft.Conn
	ccMu, scMu       sync.Mutex
	pkMu             sync.Mutex

	debugger *Debugger

	runtimeID, clientRuntimeID uint64
	uniqueID, clientUniqueID   int64

	hasClientRid, hasClientUid bool

	movementMode utils.AuthorityType
	combatMode   utils.AuthorityType

	world *world.World

	locale language.Tag

	acks *Acknowledgements

	clientTick, clientFrame, serverTick atomic.Uint64
	lastServerTicked                    time.Time

	hMutex sync.RWMutex
	h      Handler

	entity *entity.Entity

	entities sync.Map

	effects sync.Map

	mInfo *MovementInfo
	miMu  sync.Mutex

	queueMu               sync.Mutex
	queuedEntityLocations map[uint64]utils.LocationData

	gamemode  int32
	inputMode int32

	eyeOffset float32

	ready                 bool
	respawned             bool
	dead                  bool
	needsCombatValidation bool
	inDimensionChange     bool
	closed                bool

	containerID        byte
	containerOpen      bool
	containerMoveTicks int64

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

	latencyIntervalUpdate int64

	lastAttackData    *packet.InventoryTransaction
	lastEquipmentData *packet.MobEquipment
	lastActorData     *packet.SetActorData
	lastAttributeData *packet.UpdateAttributes

	nextTickActions   []func()
	nextTickActionsMu sync.Mutex

	chunkRadius      int32
	breakingBlockPos *protocol.BlockPos

	checkMu sync.Mutex
	checks  []check.Check

	toSend []packet.Packet
	tMu    sync.Mutex

	c    chan struct{}
	once sync.Once

	checkedBlacklist bool
	handleTransfer   bool
}

// NewPlayer creates a new player from the given identity data, client data, position, and world.
func NewPlayer(log *logrus.Logger, conn, serverConn *minecraft.Conn) *Player {
	data := conn.GameData()
	p := &Player{
		log: log,

		debugger: &Debugger{},

		conn:       conn,
		serverConn: serverConn,

		runtimeID: data.EntityRuntimeID,
		uniqueID:  data.EntityUniqueID,

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

		queuedEntityLocations: make(map[uint64]utils.LocationData),

		stackLatency:          0,
		latencyIntervalUpdate: 10,
		needLatencyUpdate:     true,

		gamemode: data.PlayerGameMode,

		inLoadedChunk: false,

		c: make(chan struct{}),

		toSend: make([]packet.Packet, 0),

		checks: []check.Check{
			check.NewAutoClickerA(),
			check.NewAutoClickerB(),
			check.NewAutoClickerC(),

			check.NewKillAuraA(),

			check.NewReachA(),
			check.NewReachB(),

			check.NewMovementA(),
			check.NewMovementB(),
			check.NewMovementC(),

			check.NewVelocityA(),
			check.NewVelocityB(),

			check.NewEditionFakerA(),
			check.NewEditionFakerB(),

			check.NewTimerA(),

			check.NewInvalidA(),
		},

		mInfo: &MovementInfo{
			MovementSpeed:  0.1,
			TrustFlyStatus: false,
		},

		movementMode: utils.ModeFullAuthoritative,
		combatMode:   utils.ModeFullAuthoritative,

		knockbackNetworkCutoff: DefaultNetworkLatencyCutoff,
		combatNetworkCutoff:    DefaultNetworkLatencyCutoff,

		nextTickActions: make([]func(), 0),

		world: world.NewWorld(log),
	}

	p.locale, _ = language.Parse(strings.Replace(conn.ClientData().LanguageCode, "_", "-", 1))
	p.chunkRadius = math.MaxInt16

	p.acks.UseLegacy(p.conn.Protocol().ID() < GameVersion1_20_10)
	p.acks.Refresh()

	p.mInfo.SetMaxPositionDeviations(0.3, 0.3)
	p.mInfo.SetPositionPersuasions(0.002, 0.03)

	// Any version below 1.20.10 handles NetworkStackLatency differently.

	go p.startTicking()
	return p
}

// AllowedDebug sets whether the player is allowed to run debug commands in Oomph.
func (p *Player) AllowedDebug(b bool) {
	p.debugger.AllowedDebug = b
}

// World returns the world of thte player.
func (p *Player) World() *world.World {
	return p.world
}

// SetRuntimeID sets the runtime ID of the player.
func (p *Player) SetRuntimeID(id uint64) {
	p.runtimeID = id
	if p.hasClientRid {
		return
	}

	p.hasClientRid = true
	p.clientRuntimeID = id
}

// SetUniqueID sets the unique ID of the player.
func (p *Player) SetUniqueID(id int64) {
	p.uniqueID = id
	if p.hasClientUid {
		return
	}

	p.hasClientUid = true
	p.clientUniqueID = id
}

// Conn returns the connection of the player.
func (p *Player) Conn() *minecraft.Conn {
	p.ccMu.Lock()
	defer p.ccMu.Unlock()

	return p.conn
}

// SetConn sets the connection of the player.
func (p *Player) SetConn(c *minecraft.Conn) {
	p.ccMu.Lock()
	defer p.ccMu.Unlock()

	p.conn = c
}

// ServerConn returns the server connection of the player.
func (p *Player) ServerConn() *minecraft.Conn {
	p.scMu.Lock()
	defer p.scMu.Unlock()

	return p.serverConn
}

// SetServerConn sets the server connection of the player.
func (p *Player) SetServerConn(c *minecraft.Conn) {
	p.scMu.Lock()
	defer p.scMu.Unlock()

	p.serverConn = c
}

func (p *Player) HandlePacket(pk packet.Packet, client bool) error {
	p.pkMu.Lock()
	defer p.pkMu.Unlock()

	if client {
		return p.proxyHandleClient(pk)
	}
	return p.proxyHandleServer(pk)
}

// proxyHandleClient processes a packet sent by the player.
func (p *Player) proxyHandleClient(pk packet.Packet) error {
	cancel := p.ClientProcess(pk)
	if cancel {
		return nil
	}

	if err := p.ServerConn().WritePacket(pk); err != nil {
		p.Log().Error("serverConn.WritePacket(): " + err.Error())
		if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
			p.Disconnect(disconnect.Error())
		}

		return err
	}

	return nil
}

// proxyHandleServer processes a packet sent by the server.
func (p *Player) proxyHandleServer(pk packet.Packet) error {
	cancel := p.ServerProcess(pk)
	if cancel {
		return nil
	}

	if err := p.Conn().WritePacket(pk); err != nil {
		p.Log().Error("conn.WritePacket(): " + err.Error())
		return err
	}

	return nil
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
func (p *Player) Teleport(pos mgl32.Vec3, ground bool) {
	pos = pos.Sub(mgl32.Vec3{0, 1.62})

	p.miMu.Lock()
	defer p.miMu.Unlock()

	p.mInfo.Teleporting = true
	p.mInfo.AwaitingTeleport = false
	p.mInfo.OnGround = ground
	p.mInfo.IsTeleportOnGround = ground

	p.TryDebug(fmt.Sprintf("p.Teleport(): teleported to %v", pos), DebugTypeLogged, p.debugger.LogMovement)
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

// ClientFrameSynced returns the client's simulation frame after all movement simulations
// are no longer running.
func (p *Player) ClientFrameSynced() uint64 {
	c := p.clientFrame.Load()
	if p.miMu.TryLock() {
		defer p.miMu.Unlock()
		return c
	}

	// Since a movement simulation is running, we want to apply whaever we need to
	// on the next movement simulation, and not this one.
	return c + 1
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

// Acknowledgements returns the acknowledgements of the player.
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

	return p.mInfo.TicksSinceKnockback <= 1
}

// SetKnockback updates the knockback of the player. The knockback is always sent by the server.
func (p *Player) SetKnockback(v mgl32.Vec3) {
	p.miMu.Lock()
	defer p.miMu.Unlock()

	p.mInfo.SetKnockback(v)
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

// CanExemptMovementValidation returns true if the player can exempt movement validation.
func (p *Player) CanExemptMovementValidation() bool {
	p.miMu.Lock()
	defer p.miMu.Unlock()

	return p.mInfo.CanExempt || !p.mInfo.InSupportedScenario || p.mInfo.StepClipOffset > 0
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

// SetLatencyIntervalUpdate sets the interval in ticks that the player's latency will be updated.
func (p *Player) SetLatencyIntervalUpdate(i int64) {
	p.latencyIntervalUpdate = i
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

// SetInDimensionChange sets wether the player is in a dimension change state or not.
func (p *Player) SetInDimensionChange(b bool) {
	p.inDimensionChange = b
	if p.inDimensionChange {
		p.Acknowledgements().Clear()
	}
}

// OnNextClientTick runs a function on the next input the client sends (aka. its next tick)
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

	// Send the flag event to the server if Oomph is not in direct mode.
	n1, n2 := check.Name()
	p.SendOomphEventToServer("oomph:flagged", map[string]interface{}{
		"player":     p.Name(),
		"check_main": n1,
		"check_sub":  n2,
		"violations": check.Violations(),
	})

	if now, max := check.Violations(), check.MaxViolations(); now < max {
		return
	}

	message := text.Colourf("<bold><red>Oomph detected the use of third-party software.</red></bold>")

	ctx = event.C()
	p.handler().HandlePunishment(ctx, check, &message)
	if !ctx.Cancelled() {
		p.log.Infof("%s was detected and punished for using %s%s.", p.Name(), name, variant)
		p.Disconnect(message)
	}
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
	return p.gamemode
}

// InputMode returns the input mode of the player
func (p *Player) InputMode() int32 {
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

// ShouldHandleTransfer sets if Oomph should intercept TransferPacket from the server.
func (p *Player) ShouldHandleTransfer(b bool) {
	p.handleTransfer = b
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
	l := int64(p.ServerTick()) - int64(p.ClientTick())
	if l < 0 {
		l = 0
	}

	return l
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
	p.conn.Flush()
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

		p.entities.Range(func(k, _ any) bool {
			p.entities.Delete(k)
			return true
		})

		p.effects.Range(func(k, _ any) bool {
			p.effects.Delete(k)
			return true
		})

		p.queueMu.Lock()
		maps.Clear(p.queuedEntityLocations)
		p.queueMu.Unlock()

		p.entity = nil
		p.Acknowledgements().Clear()
		close(p.c)

		p.ccMu.Lock()
		p.conn.Close()
		p.conn = nil
		p.ccMu.Unlock()

		if p.serverConn != nil {
			p.scMu.Lock()
			p.serverConn.Close()
			p.serverConn = nil
			p.scMu.Unlock()
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

// tickEntitiesPos ticks the position of all entities
func (p *Player) tickEntitiesPos() {
	sT := p.serverTick.Load()

	p.entities.Range(func(_, v any) bool {
		(v.(*entity.Entity)).TickPosition(sT)
		return true
	})
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
	if !p.ready || p.inLoadedChunkTicks <= 20 || !p.needLatencyUpdate {
		return
	}

	p.needLatencyUpdate = false
	curr := time.Now()

	p.Acknowledgement(func() {
		p.needLatencyUpdate = true
		p.stackLatency = time.Since(curr).Milliseconds()

		if !p.debugger.LogLatency {
			return
		}
		p.SendOomphDebug(fmt.Sprint("RTT + Processing Delays: ", p.stackLatency, "ms\nTick Delta: ", p.TickLatency()), packet.TextTypePopup)
	})
}

// TryTransfer attempts to transfer the player to the given address w/o disconnecting them from oomph.
func (p *Player) TryTransfer(address string) error {
	p.scMu.Lock()
	p.ccMu.Lock()
	defer p.scMu.Unlock()
	defer p.ccMu.Unlock()

	clientDat := p.conn.ClientData()
	clientDat.ServerAddress = address

	serverConn, err := minecraft.Dialer{
		IdentityData: p.conn.IdentityData(),
		ClientData:   clientDat,
		FlushRate:    -1,
	}.DialTimeout("raknet", address, time.Second*10)

	if err != nil {
		p.Log().Errorf("unable to transfer to %s: %s", address, err.Error())
		return err
	}

	oldRID := p.runtimeID
	oldUID := p.uniqueID

	data := serverConn.GameData()
	p.SetRuntimeID(data.EntityRuntimeID)
	p.SetUniqueID(data.EntityUniqueID)

	if err := serverConn.DoSpawn(); err != nil {
		p.SetRuntimeID(oldRID)
		p.SetUniqueID(oldUID)
		return err
	}

	p.World().PurgeChunks()

	p.serverConn.Close()
	p.serverConn = serverConn

	// Get all entities and remove them
	p.entities.Range(func(k, _ any) bool {
		p.conn.WritePacket(&packet.RemoveEntity{
			EntityNetworkID: k.(uint64),
		})
		p.entities.Delete(k)
		return true
	})

	// Get all effects and remove them
	p.effects.Range(func(k, _ any) bool {
		p.conn.WritePacket(&packet.MobEffect{
			EntityRuntimeID: p.clientRuntimeID,
			Operation:       packet.MobEffectRemove,
			EffectType:      k.(int32),
		})
		p.effects.Delete(k)
		return true
	})

	// Remove any weather effects
	p.conn.WritePacket(&packet.LevelEvent{
		EventType: packet.LevelEventStopThunderstorm,
		EventData: 0,
	})
	p.conn.WritePacket(&packet.LevelEvent{
		EventType: packet.LevelEventStopRaining,
		EventData: 10000,
	})

	// Set the player's gamemode
	p.conn.WritePacket(&packet.SetPlayerGameType{
		GameType: data.PlayerGameMode,
	})
	p.gamemode = data.PlayerGameMode

	// Send the player the current server's gamerules
	p.conn.WritePacket(&packet.GameRulesChanged{
		GameRules: data.GameRules,
	})
	p.conn.Flush()

	p.MovementInfo().ServerPosition = data.PlayerPosition.Sub(mgl32.Vec3{0, 1.62})
	p.MovementInfo().OnGround = true
	p.MovementInfo().Immobile = true
	p.Acknowledgement(func() {
		p.MovementInfo().Immobile = false
	})

	p.conn.WritePacket(&packet.MovePlayer{
		EntityRuntimeID: p.clientRuntimeID,
		Position:        p.mInfo.ServerPosition,
		Mode:            packet.MoveModeReset,
	})
	p.conn.WritePacket(&packet.NetworkChunkPublisherUpdate{
		Position: protocol.BlockPos{int32(p.mInfo.ServerPosition.X()) >> 4, int32(p.mInfo.ServerPosition.Z()) >> 4},
		Radius:   uint32(p.chunkRadius * 16),
	})
	p.serverConn.WritePacket(&packet.RequestChunkRadius{
		ChunkRadius:    p.chunkRadius,
		MaxChunkRadius: p.chunkRadius,
	})

	p.conn.Flush()
	p.serverConn.Flush()

	return nil
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
			p.nextSyncTick = p.ServerTick() + 20
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
			if !p.doTick() {
				return
			}
		}
	}
}

// doTick ticks the player.
func (p *Player) doTick() bool {
	p.pkMu.Lock()
	defer p.pkMu.Unlock()

	// This code calculates how much the server tick should be incremented by. This is done by checking the
	// difference between the last time the server ticked and the current time.
	delta := time.Since(p.lastServerTicked).Milliseconds()
	if delta > 100 {
		p.log.Warnf("ticking goroutine took %vms to tick", delta)
		p.serverTick.Add(uint64(delta / 50))
	} else {
		p.serverTick.Inc()
	}

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
		p.Log().Errorf("%v did not respond to acknowledgements in time", p.Name())
		p.Disconnect(game.ErrorNoAcks)
		return false
	}

	// Attempt to sync the client tick and server tick - if the client is running smoothly the difference between
	// the client and server tick shoudld remain the same.
	p.tryDoSync()
	// Update the numerical millisecond latency of the player.
	p.updateLatency()
	// Send the acknowledgement packet to the client and then flush the client's connection so that they recieve packets
	// from the server.
	p.SendAck()

	// Lock the client conn and server conn, as we need to be able to flush packets w/o new ones being processed
	// at the same time.
	p.ccMu.Lock()
	p.scMu.Lock()
	defer p.ccMu.Unlock()
	defer p.scMu.Unlock()

	// Flush the client's connection.
	if p.conn != nil {
		if err := p.conn.Flush(); err != nil {
			p.Log().Errorf("p.doTick(): unable to flush client connection: %v", err)
			return false
		}
	}

	if p.serverConn != nil {
		if err := p.serverConn.Flush(); err != nil {
			p.Log().Errorf("p.doTick(): unable to flush server connection: %v", err)
			return false
		}
	}

	p.lastServerTicked = time.Now()
	return true
}

// handler returns the handler of the player.
func (p *Player) handler() Handler {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()
	return p.h
}
