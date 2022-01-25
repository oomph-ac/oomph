package player

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/check"
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

	pos       atomic.Value
	dimension world.Dimension

	ticker *time.Ticker
	tick   uint64

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
		},
	}
	p.pos.Store(vec32to64(data.PlayerPosition))
	go p.startTicking()
	return p
}

// Move moves the player to the given position.
func (p *Player) Move(pos mgl64.Vec3) {
	p.pos.Store(p.Position().Add(pos))
	p.cleanCache()
}

// Position returns the current position of the player.
func (p *Player) Position() mgl64.Vec3 {
	return p.pos.Load().(mgl64.Vec3)
}

// ChunkPosition returns the chunk position of the player.
func (p *Player) ChunkPosition() world.ChunkPos {
	pos := p.Position()
	return world.ChunkPos{int32(math.Floor(pos[0])) >> 4, int32(math.Floor(pos[2])) >> 4}
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
			p.Move(vec32to64(pk.Position.Sub(mgl32.Vec3{0, 1.62})).Sub(p.Position()))
		}

		// Run all registered checks.
		p.checkMu.Lock()
		for _, c := range p.checks {
			c.Process(p, pk)
		}
		p.checkMu.Unlock()
	case p.serverConn:
		switch pk := pk.(type) {
		case *packet.MoveActorAbsolute:
			if pk.EntityRuntimeID == p.rid {
				p.Move(vec32to64(pk.Position.Sub(mgl32.Vec3{0, 1.62})).Sub(p.Position()))
			}
		case *packet.MovePlayer:
			if pk.EntityRuntimeID == p.rid {
				p.Move(vec32to64(pk.Position.Sub(mgl32.Vec3{0, 1.62})).Sub(p.Position()))
			}
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
	p.log.Infof("%s was flagged for %s%s! %s", p.Name(), name, variant, prettyParams(params))
	if now, max := check.Track(); now > max {
		// TODO: Event handlers.
		p.Disconnect(fmt.Sprintf("§c§lYou were banned by §6oomph§c for the reason: §6%s%s", name, variant))
	}
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

// vec32to64 converts a mgl32.Vec3 to a mgl64.Vec3.
func vec32to64(vec3 mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(vec3[0]), float64(vec3[1]), float64(vec3[2])}
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
