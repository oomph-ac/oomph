package player

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/check"
	"github.com/justtaldevelops/oomph/entity"
	"github.com/justtaldevelops/oomph/game"
	"github.com/justtaldevelops/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"math/rand"
	"sync"
	"time"
)

// Player contains information about a player, such as its virtual world.
type Player struct {
	log              *logrus.Logger
	conn, serverConn *minecraft.Conn

	rid uint64
	uid int64

	viewDist int32

	chunkMu sync.Mutex
	chunks  map[world.ChunkPos]*chunk.Chunk

	acknowledgements map[int64]func()
	ackMu            sync.Mutex

	dimension world.Dimension

	serverTicker           *time.Ticker
	clientTick, serverTick uint64

	hMutex sync.RWMutex
	h      Handler

	entity *entity.Entity

	entityMu sync.Mutex
	entities map[uint64]*entity.Entity

	queuedEntityLocations map[uint64]mgl64.Vec3

	gameMode atomic.Int32

	sneaking  atomic.Bool
	sprinting atomic.Bool

	teleporting atomic.Bool
	jumping     atomic.Bool

	immobile atomic.Bool
	flying   atomic.Bool
	dead     atomic.Bool

	clicking atomic.Bool

	clickMu       sync.Mutex
	clicks        []uint64
	lastClickTick uint64
	clickDelay    uint64
	cps           int

	checkMu sync.Mutex
	checks  []check.Check

	closed atomic.Bool
}

// NewPlayer creates a new player from the given identity data, client data, position, and world.
func NewPlayer(log *logrus.Logger, dimension world.Dimension, viewDist int32, conn, serverConn *minecraft.Conn) *Player {
	data := conn.GameData()
	p := &Player{
		log: log,

		conn:       conn,
		serverConn: serverConn,

		rid: data.EntityRuntimeID,
		uid: data.EntityUniqueID,

		viewDist: viewDist,

		chunks:    make(map[world.ChunkPos]*chunk.Chunk),
		dimension: dimension,

		h: NopHandler{},

		acknowledgements: make(map[int64]func()),

		entity: entity.NewEntity(
			game.Vec32To64(data.PlayerPosition),
			mgl64.Vec3{},
			game.Vec32To64(mgl32.Vec3{data.Pitch, data.Yaw, data.Yaw}),
			true,
		),
		entities:              make(map[uint64]*entity.Entity),
		queuedEntityLocations: make(map[uint64]mgl64.Vec3),

		gameMode: *atomic.NewInt32(serverConn.GameData().PlayerGameMode),

		serverTicker: time.NewTicker(time.Second / 20),
		checks:       check.Checks(),
	}
	go p.startTicking()

	return p
}

// Move moves the player to the given position.
func (p *Player) Move(pk *packet.PlayerAuthInput) {
	data := p.Entity()
	data.Move(game.Vec32To64(pk.Position))
	data.Rotate(mgl64.Vec3{float64(pk.Pitch), float64(pk.HeadYaw), float64(pk.Yaw)})
	data.IncrementTeleportationTicks()
	p.cleanChunks()
}

// Teleport sets the position of the player and resets the teleport ticks of the player.
func (p *Player) Teleport(pos mgl32.Vec3) {
	data := p.Entity()
	data.Move(game.Vec32To64(pos))
	data.ResetTeleportationTicks()
	p.cleanChunks()
}

// MoveActor moves an actor to the given position.
func (p *Player) MoveActor(rid uint64, pos mgl64.Vec3) {
	// If the entity exists, we can queue the location for an update.
	if _, ok := p.SearchEntity(rid); ok {
		p.queuedEntityLocations[rid] = pos
	}
}

// Entity returns the entity data of the player.
func (p *Player) Entity() *entity.Entity {
	return p.entity
}

// ServerTick returns the current server tick.
func (p *Player) ServerTick() uint64 {
	return p.serverTick
}

// ClientTick returns the current client tick. This is measured by the amount of PlayerAuthInput packets the
// client has sent. (since the packet is sent every client tick)
func (p *Player) ClientTick() uint64 {
	return p.clientTick
}

// Acknowledgement runs a function after an acknowledgement from the client.
// TODO: Stop abusing NSL!
func (p *Player) Acknowledgement(f func()) {
	t := int64(rand.Int31()) * 1000 // Ensure that we don't get screwed over because the number is too fat.
	if t < 0 {
		t *= -1
	}

	p.ackMu.Lock()
	p.acknowledgements[t] = f
	p.ackMu.Unlock()

	_ = p.conn.WritePacket(&packet.NetworkStackLatency{Timestamp: t, NeedsResponse: true})
	_ = p.conn.Flush() // Make sure we get an acknowledgement as soon as possible!
}

// Debug debugs the given check data to the console and other relevant sources.
func (p *Player) Debug(check check.Check, params map[string]interface{}) {
	name, variant := check.Name()
	ctx := event.C()
	p.handler().HandleDebug(ctx, check, params)
	ctx.Continue(func() {
		p.log.Debugf("%s (%s%s): %s", p.Name(), name, variant, utils.PrettyParameters(params))
	})
}

// Flag flags the given check data to the console and other relevant sources.
func (p *Player) Flag(check check.Check, violations float64, params map[string]interface{}) {
	if violations <= 0 {
		// No violations, don't flag anything.
		return
	}

	name, variant := check.Name()
	check.TrackViolation()

	ctx := event.C()
	p.handler().HandleFlag(ctx, check, params)
	ctx.Continue(func() {
		p.log.Infof("%s was flagged for %s%s! %s", p.Name(), name, variant, utils.PrettyParameters(params))
	})

	if now, max := check.Violations(), check.MaxViolations(); now >= max {
		ctx := event.C()
		p.handler().HandlePunishment(ctx, check)
		ctx.Continue(func() {
			p.log.Infof("%s was caught lackin for %s%s!", p.Name(), name, variant)
			p.Disconnect(fmt.Sprintf("§7[§6oomph§7] §bCaught lackin!\n§6Reason: §b%s%s", name, variant))
		})
		return
	}
}

// GameMode returns the current game mode of the player.
func (p *Player) GameMode() int32 {
	return p.gameMode.Load()
}

// Sneaking returns true if the player is currently sneaking.
func (p *Player) Sneaking() bool {
	return p.sneaking.Load()
}

// Sprinting returns true if the player is currently sprinting.
func (p *Player) Sprinting() bool {
	return p.sprinting.Load()
}

// Teleporting returns true if the player is currently teleporting.
func (p *Player) Teleporting() bool {
	return p.teleporting.Load()
}

// Jumping returns true if the player is currently jumping.
func (p *Player) Jumping() bool {
	return p.jumping.Load()
}

// Immobile returns true if the player is currently immobile.
func (p *Player) Immobile() bool {
	return p.immobile.Load()
}

// Flying returns true if the player is currently flying.
func (p *Player) Flying() bool {
	return p.flying.Load()
}

// Dead returns true if the player is currently dead.
func (p *Player) Dead() bool {
	return p.dead.Load()
}

// Clicking returns true if the player is clicking.
func (p *Player) Clicking() bool {
	return p.clicking.Load()
}

// Click adds a click to the player's click history.
func (p *Player) Click() {
	currentTick := p.ClientTick()

	p.clickMu.Lock()
	p.clicking.Store(true)
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

// Name returns the player's display name.
func (p *Player) Name() string {
	return p.IdentityData().DisplayName
}

// Disconnect disconnects the player for the reason provided.
func (p *Player) Disconnect(reason string) {
	_ = p.conn.WritePacket(&packet.Disconnect{Message: reason})
	_ = p.Close()
}

// Close closes the player.
func (p *Player) Close() error {
	if err := p.conn.Close(); err != nil {
		return err
	}

	if p.serverConn != nil {
		if err := p.serverConn.Close(); err != nil {
			return err
		}
	}

	p.serverTicker.Stop()

	p.chunkMu.Lock()
	p.chunks = nil
	p.chunkMu.Unlock()

	p.checkMu.Lock()
	p.checks = nil
	p.checkMu.Unlock()

	p.closed.Store(true)
	return nil
}

// Handle sets the handler of the player.
func (p *Player) Handle(h Handler) {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()
	p.h = h
}

// startTicking ticks the player until the connection is closed.
func (p *Player) startTicking() {
	for range p.serverTicker.C {
		p.flushEntityLocations()
		p.serverTick++
	}
}

// handler returns the handler of the player.
func (p *Player) handler() Handler {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()
	return p.h
}
