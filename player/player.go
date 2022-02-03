package player

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/physics"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/check"
	"github.com/justtaldevelops/oomph/entity"
	"github.com/justtaldevelops/oomph/omath"
	"github.com/justtaldevelops/oomph/session"
	"github.com/justtaldevelops/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
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

	dimension world.Dimension

	serverTicker *time.Ticker
	clientTick   uint64
	serverTick   uint64

	// s holds the session of the player.
	s atomic.Value

	entityMu sync.Mutex
	entities map[uint64]entity.Entity

	checkMu sync.Mutex
	checks  []check.Check

	immobile atomic.Bool
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

		acknowledgements: make(map[int64]func()),

		entities: make(map[uint64]entity.Entity),

		serverTicker: time.NewTicker(time.Second / 20),
		checks: []check.Check{
			&check.AimAssistA{},
			&check.KillAuraA{}, &check.KillAuraB{Entities: make(map[uint64]entity.Entity)},
			&check.TimerA{},
			&check.ReachA{},
			&check.AutoclickerA{}, &check.AutoclickerB{}, &check.AutoclickerC{}, &check.AutoclickerD{},
		},
	}
	p.s.Store(&session.Session{})
	p.Session().EntityData.Store(entity.Entity{
		Location: entity.Location{
			Position: omath.Vec32To64(data.PlayerPosition),
			Rotation: omath.Vec32To64(mgl32.Vec3{data.Pitch, data.Yaw, data.Yaw}),
		},
	})
	go p.startTicking()
	return p
}

// Move moves the player to the given position.
func (p *Player) Move(pos mgl64.Vec3) {
	data := p.Session().GetEntityData()
	data.LastPosition = data.Position
	data.Position = data.Position.Add(pos)
	p.Session().EntityData.Store(data)

	p.cleanCache()
}

// MoveActor moves an actor to the given position.
func (p *Player) MoveActor(rid uint64, pos mgl64.Vec3) {
	e, ok := p.Entity(rid)
	if ok {
		e.LastPosition = e.Position
		e.Position = e.Position.Add(pos)
		p.UpdateEntity(rid, e)
	}
}

// Location returns the current location of the player.
func (p *Player) Location() entity.Location {
	return p.Session().GetEntityData().Location
}

// ChunkPosition returns the chunk position of the player.
func (p *Player) ChunkPosition() world.ChunkPos {
	loc := p.Location()
	return world.ChunkPos{int32(math.Floor(loc.Position[0])) >> 4, int32(math.Floor(loc.Position[2])) >> 4}
}

// Immobile returns whether the player is immobile.
func (p *Player) Immobile() bool {
	return p.immobile.Load()
}

// ServerTick returns the current "server" tick.
func (p *Player) ServerTick() uint64 {
	return p.serverTick
}

// ClientTick returns the current client tick. This is measured by the amount of PlayerAuthInput packets the
// client has sent (since the packet is sent every client tick)
func (p *Player) ClientTick() uint64 {
	return p.clientTick
}

// Session returns the session assigned to the player.
func (p *Player) Session() *session.Session {
	return p.s.Load().(*session.Session)
}

// Acknowledgement runs a function after an acknowledgement from the client.
// TODO: Stop abusing NSL!
func (p *Player) Acknowledgement(f func()) {
	t := int64(rand.Int31()) * 1000 // ensure that we don't get screwed over because the number is too fat
	if t < 0 {
		t *= -1
	}
	_ = p.conn.WritePacket(&packet.NetworkStackLatency{Timestamp: t, NeedsResponse: true})
	p.acknowledgements[t] = f
}

// Process processes the given packet.
func (p *Player) Process(pk packet.Packet, conn *minecraft.Conn) {
	switch conn {
	case p.conn:
		if p.Session().HasFlag(session.FlagClicking) {
			p.Session().SetFlag(session.FlagClicking)
		}
		switch pk := pk.(type) {
		case *packet.NetworkStackLatency:
			if f, ok := p.acknowledgements[pk.Timestamp]; ok {
				delete(p.acknowledgements, pk.Timestamp)
				f()
			}
		case *packet.PlayerAuthInput:
			p.clientTick++
			p.Move(omath.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62})).Sub(p.Location().Position))
			if (utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) && !p.Session().HasFlag(session.FlagSneaking)) || (utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) && p.Session().HasFlag(session.FlagSneaking)) {
				p.Session().SetFlag(session.FlagSneaking)
			}
			if p.Session().HasFlag(session.FlagTeleporting) {
				p.Session().SetFlag(session.FlagTeleporting)
			}
		case *packet.LevelSoundEvent:
			if pk.SoundType == packet.SoundEventAttackNoDamage {
				p.Session().Click(p.ClientTick())
			}
		case *packet.InventoryTransaction:
			if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
				p.Session().Click(p.ClientTick())
			}
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
			p.UpdateEntity(pk.EntityRuntimeID, entity.Entity{
				Location: entity.Location{
					Position: omath.Vec32To64(pk.Position),
					Rotation: omath.Vec32To64(mgl32.Vec3{pk.Pitch, pk.Yaw, pk.HeadYaw}),
				},
			})
		case *packet.MoveActorAbsolute:
			rid := pk.EntityRuntimeID
			pos := omath.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62})).Sub(p.Location().Position)
			if rid == p.rid {
				p.Move(pos)
				p.Acknowledgement(func() {
					if utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
						p.Session().SetFlag(session.FlagTeleporting)
					}
				})
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
		case *packet.SetActorData:
			if pk.EntityRuntimeID == p.rid {
				p.Acknowledgement(func() {
					hasFlag := func(flag uint32, data int64) bool {
						return (data & (1 << (flag % 64))) > 0
					}
					var width, height float64
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]; ok {
						width = float64(f.(float32)) / 2
					}
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]; ok {
						height = float64(f.(float32))
					}
					data := p.Session().GetEntityData()
					pos := data.Position
					data.AABB = physics.NewAABB(pos.Sub(mgl64.Vec3{width, height, width}), pos.Add(mgl64.Vec3{width, height, width}))
					p.Session().EntityData.Store(data)
					if f, ok := pk.EntityMetadata[entity.DataKeyFlags]; ok {
						p.immobile.Store(hasFlag(entity.DataFlagImmobile, f.(int64)))
					}
				})
			} else {
				if e, ok := p.Entity(pk.EntityRuntimeID); ok {
					var width, height float64
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]; ok {
						width = float64(f.(float32)) / 2
					}
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]; ok {
						height = float64(f.(float32))
					}
					e.AABB = physics.NewAABB(e.Position.Sub(mgl64.Vec3{width, height, width}), e.Position.Add(mgl64.Vec3{width, height, width}))
					p.UpdateEntity(pk.EntityRuntimeID, e)
				}
			}
		case *packet.StartGame:
			p.Acknowledgement(func() {
				p.Session().Gamemode = pk.WorldGameMode
			})
		case *packet.SetPlayerGameType:
			p.Acknowledgement(func() {
				p.Session().Gamemode = pk.GameType
			})
		case *packet.SetActorMotion:
			if pk.EntityRuntimeID == p.rid {
				p.Acknowledgement(func() {
					p.Session().ServerSentMotion = pk.Velocity
					p.Session().Ticks.Motion = 0
				})
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
	if now, max := check.Violations(), check.MaxViolations(); now > float64(max) {
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

	p.serverTicker.Stop()

	p.chunkMu.Lock()
	p.chunks = nil
	p.chunkMu.Unlock()

	p.checkMu.Lock()
	p.checks = nil
	p.checkMu.Unlock()
}

// startTicking ticks the player until the connection is closed.
func (p *Player) startTicking() {
	for range p.serverTicker.C {
		p.serverTick++
		p.handleBlockTicks()
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

// vec3ToCubePos converts a mgl32.Vec3 to a cube.Pos
func vec3ToCubePos(vec mgl32.Vec3) cube.Pos {
	return cube.Pos{int(vec.X()), int(vec.Y()), int(vec.Z())}
}

// air returns the air runtime ID.
func air() uint32 {
	a, _ := chunk.StateToRuntimeID("minecraft:air", nil)
	return a
}

// handleBlockTicks should be called once every tick.
func (p *Player) handleBlockTicks() {
	var liquids, cobweb, climable uint32
	var blocks []world.Block // todo: LevelChunk::blocksInAABBB69420
	for _, v := range blocks {
		if _, ok := v.(world.Liquid); ok {
			liquids++
		} else {
			name, _ := v.EncodeBlock()
			if name == "minecraft:ladder" || name == "minecraft:vine" {
				climable++
			} else if name == "minecraft:cobweb" {
				cobweb++
			}
		}
	}

	s := p.Session()
	if liquids == 0 {
		s.Ticks.Liquid++
	} else {
		s.Ticks.Liquid = 0
	}
	if cobweb == 0 {
		s.Ticks.Cobweb++
	} else {
		s.Ticks.Cobweb = 0
	}
	if climable == 0 {
		s.Ticks.Climable++
	} else {
		s.Ticks.Climable = 0
	}
	s.Ticks.Motion++
}
