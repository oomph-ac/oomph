package player

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/check"
	"github.com/justtaldevelops/oomph/entity"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"math"
	"strings"
	"sync"
	"time"
)

// Player contains information about a player, such as its virtual world.
type Player struct {
	log              *logrus.Logger
	conn, serverConn *minecraft.Conn

	rid      uint64
	viewDist int32

	chunkMu sync.Mutex
	chunks  map[world.ChunkPos]*chunk.Chunk

	acknowledgements map[int64]func()

	loc       atomic.Value
	dimension world.Dimension

	ticker *time.Ticker
	tick   uint64

	entityMu sync.Mutex
	entities map[uint64]entity.Location

	checkMu sync.Mutex
	checks  []check.Check
}

// NewPlayer creates a new player from the given identity data, client data, position, and world.
func NewPlayer(log *logrus.Logger, dimension world.Dimension, viewDist int32, conn, serverConn *minecraft.Conn) *Player {
	data := conn.GameData()
	p := &Player{
		log: log,

		conn:       conn,
		serverConn: serverConn,

		rid:      data.EntityRuntimeID,
		viewDist: viewDist,

		chunks:    make(map[world.ChunkPos]*chunk.Chunk),
		dimension: dimension,

		ticker: time.NewTicker(time.Second / 20),
		checks: []check.Check{
			&check.TimerA{},
			&check.AimAssistA{},
			&check.KillAuraA{},
		},
	}
	p.loc.Store(entity.Location{
		Position: omath.Vec32To64(data.PlayerPosition),
		Rotation: omath.Vec32To64(mgl32.Vec3{data.Pitch, data.Yaw, data.Yaw}),
	})
	go p.startTicking()
	return p
}

// Move moves the player to the given position.
func (p *Player) Move(pos mgl64.Vec3) {
	loc := p.Location()
	loc.LastPosition = loc.Position
	loc.Position = loc.Position.Add(pos)
	p.loc.Store(loc)

	p.cleanCache()
}

// MoveActor moves an actor to the given position.
func (p *Player) MoveActor(rid uint64, pos mgl64.Vec3) {
	loc, ok := p.EntityLocation(rid)
	if ok {
		loc.LastPosition = loc.Position
		loc.Position = loc.Position.Add(pos)
		p.UpdateLocation(rid, loc)
	}
}

// Location returns the current location of the player.
func (p *Player) Location() entity.Location {
	return p.loc.Load().(entity.Location)
}

// ChunkPosition returns the chunk position of the player.
func (p *Player) ChunkPosition() world.ChunkPos {
	loc := p.Location()
	return world.ChunkPos{int32(math.Floor(loc.Position[0])) >> 4, int32(math.Floor(loc.Position[2])) >> 4}
}

// Tick returns the current tick of the player.
func (p *Player) Tick() uint64 {
	return p.tick
}

// Acknowledgement runs a function after an acknowledgement from the client.
// TODO: Stop abusing NSL!
func (p *Player) Acknowledgement(f func()) {
	t := time.Now().UnixMilli()
	_ = p.conn.WritePacket(&packet.NetworkStackLatency{Timestamp: t, NeedsResponse: true})
	p.acknowledgements[t] = f
}

// Process processes the given packet.
func (p *Player) Process(pk packet.Packet, conn *minecraft.Conn) {
	switch conn {
	case p.conn:
		switch pk := pk.(type) {
		case *packet.NetworkStackLatency:
			if f, ok := p.acknowledgements[pk.Timestamp]; ok {
				delete(p.acknowledgements, pk.Timestamp)
				f()
			}
		case *packet.PlayerAuthInput:
			p.Move(omath.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62})).Sub(p.Location().Position))
		}

		// Run all registered checks.
		p.checkMu.Lock()
		for _, c := range p.checks {
			c.Process(p, pk)
		}
		p.checkMu.Unlock()
	case p.serverConn:
		switch pk := pk.(type) {
		case *packet.AddPlayer:
			p.UpdateLocation(pk.EntityRuntimeID, entity.Location{
				Position: omath.Vec32To64(pk.Position),
				Rotation: omath.Vec32To64(mgl32.Vec3{pk.Pitch, pk.Yaw, pk.HeadYaw}),
			})
		case *packet.MoveActorAbsolute:
			rid := pk.EntityRuntimeID
			pos := omath.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62})).Sub(p.Location().Position)
			if rid == p.rid {
				p.Move(pos)
				return
			}
			p.MoveActor(rid, pos)
		case *packet.LevelChunk:
			p.LoadRawChunk(world.ChunkPos{pk.ChunkX, pk.ChunkZ}, pk.RawPayload, pk.SubChunkCount)
		case *packet.UpdateBlock:
			block, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
			if ok {
				p.SetBlock(protocolPosToCubePos(pk.Position), block)
			}
		}
	}
}

// Debug debugs the given check data to the console and other relevant sources.
func (p *Player) Debug(check check.Check, params ...map[string]interface{}) {
	name, variant := check.Name()
	p.log.Debugf("%s (%s%s): %s", p.Name(), name, variant, prettyParams(params))
}

// Flag flags the given check data to the console and other relevant sources.
func (p *Player) Flag(check check.Check, params ...map[string]interface{}) {
	name, variant := check.Name()
	check.TrackViolation()
	if now, max := check.Violations(), check.MaxViolations(); now > max {
		// TODO: Event handlers.
		p.Disconnect(fmt.Sprintf("§7[§6oomph§7] §bCaught lackin!\n§6Reason: §b%s%s", name, variant))
	}

	p.log.Infof("%s was flagged for %s%s! %s", p.Name(), name, variant, prettyParams(params))
}

// Name ...
func (p *Player) Name() string {
	return p.conn.IdentityData().DisplayName
}

// Disconnect disconnects the player for the reason provided.
func (p *Player) Disconnect(reason string) {
	_ = p.conn.WritePacket(&packet.Disconnect{Message: reason})
	p.Close()
}

// Close closes the player.
func (p *Player) Close() {
	_, _ = p.conn.Flush(), p.serverConn.Flush()
	_, _ = p.conn.Close(), p.serverConn.Close()

	p.ticker.Stop()

	p.chunkMu.Lock()
	p.chunks = nil
	p.chunkMu.Unlock()

	p.checkMu.Lock()
	p.checks = nil
	p.checkMu.Unlock()
}

// startTicking ticks the player until the connection is closed.
func (p *Player) startTicking() {
	for range p.ticker.C {
		p.tick++
	}
}

// prettyParams converts the given parameters to a readable string.
func prettyParams(params []map[string]interface{}) string {
	if len(params) == 0 {
		// Don't waste our time if there are no parameters.
		return "[]"
	}
	// Hacky but simple way to create a readable string.
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(fmt.Sprint(params[0]), "map"), " ", ", "), ":", "=")
}

// protocolPosToCubePos converts a protocol.BlockPos to a cube.Pos.
func protocolPosToCubePos(pos protocol.BlockPos) cube.Pos {
	return cube.Pos{int(pos.X()), int(pos.Y()), int(pos.Z())}
}

// air returns the air runtime ID.
func air() uint32 {
	a, _ := chunk.StateToRuntimeID("minecraft:air", nil)
	return a
}
