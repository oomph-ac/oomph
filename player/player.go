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
	"github.com/justtaldevelops/oomph/settings"
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

	settings settings.Settings

	chunkMu sync.Mutex
	chunks  map[world.ChunkPos]*chunk.Chunk

	acknowledgements map[int64]func()
	ackMu            sync.Mutex

	dimension world.Dimension

	serverTicker *time.Ticker
	clientTick   uint64
	serverTick   uint64

	hMutex sync.RWMutex
	h      Handler

	entityMu sync.Mutex
	entities map[uint64]entity.Entity

	queuedEntityLocations map[uint64]mgl64.Vec3

	checkMu sync.Mutex
	checks  []check.Check

	closed atomic.Bool
}

// NewPlayer creates a new player from the given identity data, client data, position, and world.
func NewPlayer(log *logrus.Logger, dimension world.Dimension, settings settings.Settings, conn, serverConn *minecraft.Conn) *Player {
	data := conn.GameData()
	p := &Player{
		log: log,

		conn:       conn,
		serverConn: serverConn,

		rid: data.EntityRuntimeID,
		uid: data.EntityUniqueID,

		settings: settings,

		chunks:    make(map[world.ChunkPos]*chunk.Chunk),
		dimension: dimension,

		h: NopHandler{},

		acknowledgements: make(map[int64]func()),

		entities:              make(map[uint64]entity.Entity),
		queuedEntityLocations: make(map[uint64]mgl64.Vec3),

		serverTicker: time.NewTicker(time.Second / 20),
		checks: []check.Check{
			check.NewAimAssistA(),
			check.NewAutoClickerA(), check.NewAutoClickerB(), check.NewAutoClickerC(), check.NewAutoClickerD(),
			check.NewInvalidMovementC(),
			check.NewKillAuraA(), check.NewKillAuraB(),
			check.NewReachA(),
			check.NewTimerA(),
		},
	}

	// TODO: Make a check registry.
	var checks []check.Check
	for _, c := range p.checks {
		if c.BaseSettings().Enabled {
			checks = append(checks, c)
		}
	}
	p.checks = checks

	// Validate device OS
	osCheck := &check.OSSpoofer{GivenOS: conn.ClientData().DeviceOS, TitleID: conn.IdentityData().TitleID}
	if osCheck.BaseSettings().Enabled {
		osCheck.Process(p, nil)
	}

	p.entity = entity.Entity{
		Position: minecraft.Vec32To64(data.PlayerPosition),
		Rotation: minecraft.Vec32To64(mgl32.Vec3{data.Pitch, data.Yaw, data.Yaw}),
	}
	go p.startTicking()
	return p
}

// Move moves the player to the given position.
func (p *Player) Move(pk *packet.PlayerAuthInput) {
	s := p.Session()
	data := s.Entity()
	data.LastPosition = data.Position
	data.Position = minecraft.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62}))
	data.LastRotation = data.Rotation
	data.Rotation = mgl64.Vec3{float64(pk.Pitch), float64(pk.HeadYaw), float64(pk.Yaw)}
	data.TeleportTicks++
	s.EntityData.Store(data)
	s.Movement.Motion = data.Position.Sub(data.LastPosition)

	p.cleanChunks()
}

// Teleport sets the position of the player and resets the teleport ticks
func (p *Player) Teleport(pos mgl32.Vec3) {
	data := p.Session().Entity()
	data.LastPosition = data.Position
	data.Position = minecraft.Vec32To64(pos.Sub(mgl32.Vec3{0, 1.62}))
	data.TeleportTicks = 0
	p.Session().EntityData.Store(data)

	p.cleanChunks()
}

// MoveActor moves an actor to the given position.
func (p *Player) MoveActor(rid uint64, pos mgl64.Vec3) {
	// If the entity exists, we can queue the location for an update.
	if _, ok := p.Entity(rid); ok {
		p.queuedEntityLocations[rid] = pos
	}
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

	if now, max := check.Violations(), check.BaseSettings().MaxViolations; now >= float64(max) {
		ctx := event.C()
		p.handler().HandlePunishment(ctx, check)
		ctx.Continue(func() {
			p.log.Infof("%s was caught lackin for %s%s!", p.Name(), name, variant)
			p.Disconnect(fmt.Sprintf("§7[§6oomph§7] §bCaught lackin!\n§6Reason: §b%s%s", name, variant))
		})
		return
	}
}

// Name returns the player's display name.
func (p *Player) Name() string {
	return p.IdentityData().DisplayName
}

// Disconnect disconnects the player for the reason provided.
func (p *Player) Disconnect(reason string) {
	_ = p.conn.WritePacket(&packet.Disconnect{Message: reason})
	p.Close()
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
	p.h = h
	p.hMutex.Unlock()
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
