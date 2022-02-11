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

	ackMu            sync.Mutex
	acknowledgements map[int64]func()

	dimension world.Dimension

	serverTicker *time.Ticker
	clientTick   uint64
	serverTick   uint64

	// s holds the session of the player.
	s atomic.Value

	entityMu              sync.Mutex
	entities              map[uint64]entity.Entity
	queuedEntityLocations map[uint64]mgl64.Vec3

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

		entities:              make(map[uint64]entity.Entity),
		queuedEntityLocations: make(map[uint64]mgl64.Vec3),

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
func (p *Player) Move(pk *packet.PlayerAuthInput) {
	data := p.Session().GetEntityData()
	data.LastPosition = data.Position
	data.Position = omath.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62, 0}))
	data.AABB = physics.NewAABB(data.Position.Sub(mgl64.Vec3{data.BBWidth, 0, data.BBWidth}), data.Position.Add(mgl64.Vec3{data.BBWidth, data.BBHeight, data.BBWidth}))
	data.LastRotation = data.Rotation
	data.Rotation = mgl64.Vec3{float64(pk.Pitch), float64(pk.Yaw), float64(pk.HeadYaw)}
	data.TeleportTicks++
	p.Session().EntityData.Store(data)

	p.cleanCache()
}

// Teleport sets the position of the player and resets the teleport ticks
func (p *Player) Teleport(pk *packet.MoveActorAbsolute) {
	data := p.Session().GetEntityData()
	data.LastPosition = data.Position
	data.Position = omath.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62}))
	data.TeleportTicks = 0
	p.Session().EntityData.Store(data)

	p.cleanCache()
}

// MoveActor moves an actor to the given position.
func (p *Player) MoveActor(rid uint64, pos mgl64.Vec3) {
	_, ok := p.Entity(rid)
	if ok {
		// if the entity is valid, we can queue the location for an update
		p.queueEntityLocation(rid, pos)
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

// SendAcknowledgement runs a function after an acknowledgement from the client.
// TODO: Stop abusing NSL!
func (p *Player) SendAcknowledgement(f func()) {
	t := int64(rand.Int31()) * 1000 // ensure that we don't get screwed over because the number is too fat
	if t < 0 {
		t *= -1
	}
	_ = p.conn.WritePacket(&packet.NetworkStackLatency{Timestamp: t, NeedsResponse: true})
	p.ackMu.Lock()
	p.acknowledgements[t] = f
	p.ackMu.Unlock()
}

// handleAcknowledgement handles an acknowledgement function in the acknowledgement map
func (p *Player) handleAcknowledgement(t int64) {
	p.ackMu.Lock()
	call, ok := p.acknowledgements[t]
	if ok {
		call()
		delete(p.acknowledgements, t)
	}
	p.ackMu.Unlock()
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
			p.handleAcknowledgement(pk.Timestamp)
		case *packet.PlayerAuthInput:
			p.clientTick++
			p.Move(pk)
			if (utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) && !p.Session().HasFlag(session.FlagSneaking)) || (utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) && p.Session().HasFlag(session.FlagSneaking)) {
				p.Session().SetFlag(session.FlagSneaking)
			}
			if p.Session().HasFlag(session.FlagTeleporting) {
				p.Session().SetFlag(session.FlagTeleporting)
			}
			p.handleBlockTicks()
			p.tickEntityLocations()
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
			if pk.EntityRuntimeID == p.rid {
				return
			}
			p.SendAcknowledgement(func() {
				p.UpdateEntity(pk.EntityRuntimeID, entity.Entity{
					Location: entity.Location{
						Position:                 omath.Vec32To64(pk.Position),
						LastPosition:             omath.Vec32To64(pk.Position),
						RecievedPosition:         omath.Vec32To64(pk.Position).Add(omath.Vec32To64(pk.Velocity)),
						NewPosRotationIncrements: 3,
						Rotation:                 omath.Vec32To64(mgl32.Vec3{pk.Pitch, pk.Yaw, pk.HeadYaw}),
					},
					AABB: physics.NewAABB(
						omath.Vec32To64(pk.Position).Sub(mgl64.Vec3{0.3, 0, 0.3}),
						omath.Vec32To64(pk.Position).Add(mgl64.Vec3{0.3, 1.8, 0.3}),
					),
					BBWidth:  0.3,
					BBHeight: 1.8,
					IsPlayer: true,
				})
			})
		case *packet.AddActor:
			if pk.EntityRuntimeID == p.rid {
				return
			}
			p.SendAcknowledgement(func() {
				p.UpdateEntity(pk.EntityRuntimeID, entity.Entity{
					Location: entity.Location{
						Position:                 omath.Vec32To64(pk.Position),
						LastPosition:             omath.Vec32To64(pk.Position),
						RecievedPosition:         omath.Vec32To64(pk.Position).Add(omath.Vec32To64(pk.Velocity)),
						NewPosRotationIncrements: 3,
						Rotation:                 omath.Vec32To64(mgl32.Vec3{pk.Pitch, pk.Yaw, pk.HeadYaw}),
					},
					AABB: physics.NewAABB(
						omath.Vec32To64(pk.Position).Sub(mgl64.Vec3{0.3, 0, 0.3}),
						omath.Vec32To64(pk.Position).Add(mgl64.Vec3{0.3, 1.8, 0.3}),
					),
					BBWidth:  0.3,
					BBHeight: 1.8,
					IsPlayer: false,
				})
			})
		case *packet.MoveActorAbsolute:
			rid := pk.EntityRuntimeID
			if rid == p.rid {
				p.SendAcknowledgement(func() {
					p.Teleport(pk)
					if utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
						p.Session().SetFlag(session.FlagTeleporting)
					}
				})
				return
			}
			p.MoveActor(rid, omath.Vec32To64(pk.Position))
		case *packet.LevelChunk:
			p.SendAcknowledgement(func() {
				p.LoadRawChunk(world.ChunkPos{pk.ChunkX, pk.ChunkZ}, pk.RawPayload, pk.SubChunkCount)
			})
		case *packet.UpdateBlock:
			block, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
			if ok {
				p.SendAcknowledgement(func() {
					p.SetBlock(protocolPosToCubePos(pk.Position), block)
				})
			}
		case *packet.SetActorData:
			hasFlag := func(flag uint32, data int64) bool {
				return (data & (1 << (flag % 64))) > 0
			}
			if pk.EntityRuntimeID == p.rid {
				p.SendAcknowledgement(func() {
					data := p.Session().GetEntityData()
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]; ok {
						data.BBWidth = float64(f.(float32)) / 2
					}
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]; ok {
						data.BBHeight = float64(f.(float32))
					}
					p.Session().EntityData.Store(data)
					if f, ok := pk.EntityMetadata[entity.DataKeyFlags]; ok {
						p.immobile.Store(hasFlag(entity.DataFlagImmobile, f.(int64)))
					}
				})
			} else {
				p.SendAcknowledgement(func() {
					if e, ok := p.Entity(pk.EntityRuntimeID); ok {
						if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]; ok {
							e.BBWidth = float64(f.(float32)) / 2
						}
						if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]; ok {
							e.BBHeight = float64(f.(float32))
						}
						p.UpdateEntity(pk.EntityRuntimeID, e)
					}
				})
			}
		case *packet.StartGame:
			p.SendAcknowledgement(func() {
				p.Session().Gamemode = pk.WorldGameMode
			})
		case *packet.SetPlayerGameType:
			p.SendAcknowledgement(func() {
				p.Session().Gamemode = pk.GameType
			})
		case *packet.SetActorMotion:
			if pk.EntityRuntimeID == p.rid {
				p.SendAcknowledgement(func() {
					p.Session().ServerSentMotion = pk.Velocity
					p.Session().Ticks.Motion = 0
				})
			}
		case *packet.RemoveActor:
			p.SendAcknowledgement(func() {
				p.RemoveEntity(uint64(pk.EntityUniqueID))
			})
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
		p.flushEntityLocations()
		p.conn.Flush() // make sure the network stack latency packet gets to the client ASAP
		p.serverTick++
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

// queueEntityLocation is called when an entity location should be queued to update
// and lag compensate with NSL
func (p *Player) queueEntityLocation(rid uint64, pos mgl64.Vec3) {
	p.queuedEntityLocations[rid] = pos
}

// tickEntityLocations ticks entity locations to simulate what the client would see for the
func (p *Player) tickEntityLocations() {
	for eid := range p.entities {
		e, _ := p.Entity(eid)
		if e.NewPosRotationIncrements > 0 {
			delta := e.RecievedPosition.Sub(e.LastPosition).Mul(1 / float64(e.NewPosRotationIncrements))
			e.LastPosition = e.Position
			e.Position = e.Position.Add(delta)
			e.AABB = physics.NewAABB(
				e.Position.Sub(mgl64.Vec3{e.BBWidth, 0, e.BBWidth}),
				e.Position.Add(mgl64.Vec3{e.BBWidth, e.BBHeight, e.BBWidth}),
			)
			e.NewPosRotationIncrements--
		}
		e.TeleportTicks++
		p.UpdateEntity(eid, e)
	}
}

// flushEntityLocations clears the queued entity location map, and sends an acknowledgement to the player
// This allows us to know when the client has recieved positions of other entities
func (p *Player) flushEntityLocations() {
	queue := p.queuedEntityLocations
	p.queuedEntityLocations = make(map[uint64]mgl64.Vec3)
	p.SendAcknowledgement(func() {
		for rid, pos := range queue {
			if e, valid := p.Entity(rid); valid {
				e.RecievedPosition = pos
				e.NewPosRotationIncrements = 3
				p.UpdateEntity(rid, e)
			}
		}
	})
}

// handleBlockTicks is called once every client tick to update block ticks
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
