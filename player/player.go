package player

import (
	"fmt"
	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/check"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Player contains information about a player, such as its virtual world or AABB.
type Player struct {
	log              *logrus.Logger
	conn, serverConn *minecraft.Conn

	rid uint64
	uid int64

	locale language.Tag

	ackMu            sync.Mutex
	acknowledgements map[int64]func()

	clientTick, serverTick atomic.Uint64

	hMutex sync.RWMutex
	h      Handler

	entity *entity.Entity

	entityMu sync.Mutex
	entities map[uint64]*entity.Entity

	queueMu               sync.Mutex
	queuedEntityLocations map[uint64]mgl64.Vec3

	ready    bool
	gameMode int32

	sneaking, sprinting    bool
	teleporting, jumping   bool
	immobile, flying, dead bool

	clickMu       sync.Mutex
	clicking      bool
	clicks        []uint64
	lastClickTick uint64
	clickDelay    uint64
	cps           int

	checkMu sync.Mutex
	checks  []check.Check

	c    chan struct{}
	once sync.Once

	world.NopViewer
}

// NewPlayer creates a new player from the given identity data, client data, position, and world.
func NewPlayer(log *logrus.Logger, conn, serverConn *minecraft.Conn) *Player {
	data := conn.GameData()
	p := &Player{
		log: log,

		conn:       conn,
		serverConn: serverConn,

		rid: data.EntityRuntimeID,
		uid: data.EntityUniqueID,

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

		gameMode: data.PlayerGameMode,

		c: make(chan struct{}),

		checks: []check.Check{
			check.NewAimAssistA(),

			check.NewAutoClickerA(),
			check.NewAutoClickerB(),

			check.NewAutoClickerC(),
			check.NewAutoClickerD(),

			check.NewInvalidMovementC(),

			check.NewKillAuraA(),
			check.NewKillAuraB(),

			check.NewOSSpoofer(),

			check.NewReachA(),

			check.NewTimerA(),
		},
	}
	p.locale, _ = language.Parse(strings.Replace(conn.ClientData().LanguageCode, "_", "-", 1))
	go p.startTicking()
	return p
}

// Conn returns the connection of the player.
func (p *Player) Conn() *minecraft.Conn {
	return p.conn
}

// Move moves the player to the given position.
func (p *Player) Move(pk *packet.PlayerAuthInput) {
	data := p.Entity()
	data.Move(game.Vec32To64(pk.Position), true)
	data.Rotate(mgl64.Vec3{float64(pk.Pitch), float64(pk.HeadYaw), float64(pk.Yaw)})
	data.IncrementTeleportationTicks()
}

// Teleport sets the position of the player and resets the teleport ticks of the player.
func (p *Player) Teleport(pos mgl32.Vec3, reset bool) {
	data := p.Entity()
	data.Move(game.Vec32To64(pos), true)
	if reset {
		data.ResetTeleportationTicks()
	} else {
		data.IncrementTeleportationTicks()
	}
}

// MoveEntity moves an entity to the given position.
func (p *Player) MoveEntity(rid uint64, pos mgl64.Vec3) {
	// If the entity exists, we can queue the location for an update.
	if _, ok := p.SearchEntity(rid); ok {
		p.queueMu.Lock()
		p.queuedEntityLocations[rid] = pos
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

// ClientTick returns the current client tick. This is measured by the amount of PlayerAuthInput packets the
// client has sent. (since the packet is sent every client tick)
func (p *Player) ClientTick() uint64 {
	return p.clientTick.Load()
}

// Position returns the position of the player.
func (p *Player) Position() mgl64.Vec3 {
	return p.Entity().Position()
}

// Rotation returns the rotation of the player.
func (p *Player) Rotation() mgl64.Vec3 {
	return p.Entity().Rotation()
}

// AABB returns the axis-aligned bounding box of the player.
func (p *Player) AABB() cube.BBox {
	return p.Entity().AABB()
}

// Acknowledgement runs a function after an acknowledgement from the client.
// TODO: Stop abusing NSL!
func (p *Player) Acknowledgement(f func()) {
	if p.acknowledgements == nil {
		// Don't request an acknowledgement if acknowledgements are closed.
		return
	}

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
	check.TrackViolation()

	ctx := event.C()
	p.handler().HandleFlag(ctx, check, params)
	if ctx.Cancelled() {
		return
	}
	p.log.Infof("%s was flagged for %s%s: %s", p.Name(), name, variant, utils.PrettyParameters(params, true))
	if now, max := check.Violations(), check.MaxViolations(); now >= max {
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
}

// Ready returns true if the player is ready/spawned in.
func (p *Player) Ready() bool {
	return p.ready
}

// GameMode returns the current game mode of the player.
func (p *Player) GameMode() int32 {
	return p.gameMode
}

// Sneaking returns true if the player is currently sneaking.
func (p *Player) Sneaking() bool {
	return p.sneaking
}

// Sprinting returns true if the player is currently sprinting.
func (p *Player) Sprinting() bool {
	return p.sprinting
}

// Teleporting returns true if the player is currently teleporting.
func (p *Player) Teleporting() bool {
	return p.teleporting
}

// Jumping returns true if the player is currently jumping.
func (p *Player) Jumping() bool {
	return p.jumping
}

// Immobile returns true if the player is currently immobile.
func (p *Player) Immobile() bool {
	return p.immobile
}

// Flying returns true if the player is currently flying.
func (p *Player) Flying() bool {
	return p.flying
}

// Dead returns true if the player is currently dead.
func (p *Player) Dead() bool {
	return p.dead
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
	p.once.Do(func() {
		p.checkMu.Lock()
		p.checks = nil
		p.checkMu.Unlock()

		p.ackMu.Lock()
		p.acknowledgements = nil
		p.ackMu.Unlock()

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

// startTicking ticks the player until the connection is closed.
func (p *Player) startTicking() {
	t := time.NewTicker(time.Second / 20)
	defer t.Stop()
	for {
		select {
		case <-p.c:
			return
		case <-t.C:
			p.flushEntityLocations()
			p.serverTick.Inc()
		}
	}
}

// handler returns the handler of the player.
func (p *Player) handler() Handler {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()
	return p.h
}
