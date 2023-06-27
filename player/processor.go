package player

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "unsafe"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/event"

	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var air uint32

func init() {
	var found bool
	air, found = chunk.StateToRuntimeID("minecraft:air", nil)
	if !found {
		panic("can't find air runtime id!")
	}
}

// ClientProcess processes a given packet from the client.
func (p *Player) ClientProcess(pk packet.Packet) bool {
	cancel := false
	p.clicking = false

	defer func() {
		if cancel {
			return
		}

		ctx := event.C()
		p.handler().HandleClientPacket(ctx, pk)
		cancel = ctx.Cancelled()
	}()

	if p.closed {
		return false
	}

	switch pk := pk.(type) {
	case *packet.Disconnect:
		p.Close()
		return false
	case *packet.TickSync:
		// The tick sync packet is sent once by the client on join and the server responds with another.
		// From what I can tell, the client frame is supposed to be rewound to `ServerReceptionTime` in the
		// tick sync sent by the server, but as of now it seems to be useless.
		// To replicate the same behaviour, we get the current "server" tick, and
		// send an acknowledgement to the client (replicating when the client receives a TickSync from the server)
		// and once the client responds, the client tick (not frame) value is set to the server tick the acknowledgement
		// was sent in.
		curr := p.serverTick.Load()
		p.Acknowledgement(func() {
			p.clientTick.Store(curr)
			p.isSyncedWithServer = true
			p.Acknowledgement(func() {
				p.ready = true
			})
		})
		p.rid = p.conn.GameData().EntityRuntimeID
	case *packet.NetworkStackLatency:
		cancel = p.Acknowledgements().Handle(pk.Timestamp, p.ClientData().DeviceOS == protocol.DeviceOrbis)
	case *packet.PlayerAuthInput:
		p.clientTick.Inc()
		p.clientFrame.Store(pk.Tick)

		p.cleanChunks()
		p.handlePlayerAuthInput(pk)

		if p.combatMode == utils.ModeFullAuthoritative {
			p.validateCombat()
		} else {
			p.tickEntitiesPos()
		}

		if acks := p.Acknowledgements(); acks != nil {
			acks.HasTicked = true
		}
		p.needsCombatValidation = false

		if p.debugger.Chunks && p.ClientTick()%20 == 0 {
			p.SendOomphDebug(fmt.Sprint("pos=", game.RoundVec32(p.mInfo.ServerPosition, 2), " loaded=", p.inLoadedChunk), packet.TextTypeChat)
		}

		defer p.SetRespawned(false)
		if p.movementMode == utils.ModeSemiAuthoritative {
			defer p.setMovementToClient()
		}
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			p.Click()
			p.updateCombatData(nil)
		}
	case *packet.MobEquipment:
		p.lastEquipmentData = pk
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			cancel = p.combatMode == utils.ModeFullAuthoritative
			p.updateCombatData(pk)
			p.Click()
		} else if t, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok && t.ActionType == protocol.UseItemActionClickBlock {
			cancel = p.handleBlockPlace(t)
		}
	case *packet.Text:
		cmd := strings.Split(pk.Message, " ")
		if cmd[0] == ".oomph_debug" {
			if len(cmd) != 3 {
				p.SendOomphDebug("Usage: .oomph_debug <mode> <value>", packet.TextTypeChat)
				return true
			}

			b := cmd[2] == "on" || cmd[2] == "true"

			switch cmd[1] {
			case "latency":
				p.debugger.Latency = b
			case "server_combat":
				p.debugger.ServerCombat = b
			case "server_knockback":
				p.debugger.ServerKnockback = b
			case "buffer_info":
				p.debugger.PacketBuffer = b
			case "packet_buffer":
				p.UsePacketBuffering(b)
			case "game_speed":
				// convert to float32 using strconv.ParseFloat
				f, err := strconv.ParseFloat(cmd[2], 32)
				if err != nil {
					p.SendOomphDebug("Invalid value: "+cmd[2], packet.TextTypeChat)
					return true
				}

				pk := &packet.LevelEvent{
					EventType: packet.LevelEventSimTimeScale,
					Position:  mgl32.Vec3{1},
				}

				p.conn.WritePacket(pk)
				pk.Position = mgl32.Vec3{float32(f)}
				p.conn.WritePacket(pk)
			case "movement":
				p.debugger.Movement = b
			case "chunks":
				p.debugger.Chunks = b
			default:
				p.SendOomphDebug("Unknown debug mode: "+cmd[1], packet.TextTypeChat)
			}
			p.SendOomphDebug("OK.", packet.TextTypeChat)
			return true
		}

		if p.serverConn != nil {
			// Strip the XUID to prevent certain server software from flagging the message as spam.
			pk.XUID = ""
		}
	}

	// Run all registered checks.
	p.checkMu.Lock()
	defer p.checkMu.Unlock()
	for _, c := range p.checks {
		cancel = c.Process(p, pk) || cancel
	}

	return cancel
}

// ServerProcess processes a given packet from the server.
func (p *Player) ServerProcess(pk packet.Packet) (cancel bool) {
	if p.closed {
		return false
	}

	defer func() {
		if cancel {
			return
		}

		ctx := event.C()
		p.handler().HandleClientPacket(ctx, pk)
		cancel = ctx.Cancelled()
	}()

	switch pk := pk.(type) {
	case *packet.AddPlayer:
		if pk.EntityRuntimeID == p.rid {
			// We are the player.
			return false
		}

		p.Acknowledgement(func() {
			p.AddEntity(pk.EntityRuntimeID, entity.NewEntity(
				pk.Position,
				pk.Velocity,
				mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw},
				true,
			))
		})
	case *packet.AddActor:
		if pk.EntityRuntimeID == p.rid {
			// We are the player.
			return false
		}

		p.Acknowledgement(func() {
			p.AddEntity(pk.EntityRuntimeID, entity.NewEntity(
				pk.Position,
				pk.Velocity,
				mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw},
				false,
			))
		})
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.rid {
			p.MoveEntity(pk.EntityRuntimeID, pk.Position, utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport), utils.HasFlag(uint64(pk.Flags), packet.MoveFlagOnGround))
			return false
		}

		if !utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
			return false
		}

		p.Acknowledgement(func() {
			p.Teleport(pk.Position)
		})
	case *packet.MovePlayer:
		if pk.EntityRuntimeID != p.rid {
			p.MoveEntity(pk.EntityRuntimeID, pk.Position, pk.Mode == packet.MoveModeTeleport, pk.OnGround)
			return false
		}

		// If rewind is being applied with this packet, teleport the player instantly on the server
		// instead of waiting for the client to acknowledge the packet.
		if p.movementMode == utils.ModeFullAuthoritative {
			pk.Tick = p.ClientFrame()
			p.Teleport(pk.Position)
			return false
		} else {
			pk.Tick = 0
		}

		p.Acknowledgement(func() {
			p.Teleport(pk.Position)
		})
	case *packet.SetActorData:
		pk.Tick = 0 // prevent rewind from happening

		if pk.EntityRuntimeID == p.rid {
			p.lastSentActorData = pk
		}

		p.Acknowledgement(func() {
			p.handleSetActorData(pk)
		})
	case *packet.SetPlayerGameType:
		p.Acknowledgement(func() {
			p.gameMode = pk.GameType
		})
	case *packet.RemoveActor:
		if pk.EntityUniqueID == p.uid {
			return false
		}

		p.Acknowledgement(func() {
			p.RemoveEntity(uint64(pk.EntityUniqueID))
		})
	case *packet.UpdateAttributes:
		pk.Tick = 0 // prevent any rewind from being done
		if pk.EntityRuntimeID != p.rid {
			return false
		}

		for _, a := range pk.Attributes {
			if a.Name == "minecraft:health" && a.Value <= 0 {
				p.isSyncedWithServer = false
				p.dead = true

				p.chkMu.Lock()
				p.chunks = make(map[protocol.ChunkPos]*ChunkSubscriptionInfo)
				p.inLoadedChunk = false
				p.inLoadedChunkTicks = 0
				p.chkMu.Unlock()

				break
			}
		}

		p.lastSentAttributes = pk
		p.Acknowledgement(func() {
			for _, a := range pk.Attributes {
				if a.Name == "minecraft:movement" {
					p.miMu.Lock()
					p.mInfo.Speed = a.Value
					p.miMu.Unlock()
					break
				}
			}
		})
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.rid {
			return false
		}

		// If the player is behind by more than 6 ticks (300ms), then instantly set the KB
		// of the player instead of waiting for an acknowledgement. This will ensure that players
		// with very high latency do not get a significant advantage due to them receiving knockback late.
		if (p.movementMode == utils.ModeFullAuthoritative && p.TickLatency() >= NetworkLatencyCutoff) || p.debugger.ServerKnockback {
			p.UpdateServerVelocity(pk.Velocity)
			return false
		}

		// Send an acknowledgement to the player to get the client tick where the player will apply KB and verify that the client
		// does take knockback when it recieves it.
		p.Acknowledgement(func() {
			p.UpdateServerVelocity(pk.Velocity)
		})
	case *packet.LevelChunk:
		p.handleLevelChunk(pk)
	case *packet.SubChunk:
		switch p.movementMode {
		case utils.ModeSemiAuthoritative:
			p.Acknowledgement(func() {
				p.handleSubChunk(pk)
			})
		case utils.ModeFullAuthoritative:
			p.handleSubChunk(pk)
		}
	case *packet.ChunkRadiusUpdated:
		p.chunkRadius = pk.ChunkRadius + 4
	case *packet.UpdateBlock:
		if p.movementMode == utils.ModeClientAuthoritative {
			return false
		}

		b, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			return false
		}

		chunkPos := GetChunkPos(pk.Position[0], pk.Position[2])
		p.makeChunkCopy(chunkPos)

		if p.movementMode == utils.ModeFullAuthoritative {
			p.SetBlock(utils.BlockToCubePos(pk.Position), b)
			return false
		}

		p.Acknowledgement(func() {
			p.SetBlock(utils.BlockToCubePos(pk.Position), b)
		})
	case *packet.MobEffect:
		if pk.EntityRuntimeID != p.rid {
			return false
		}

		p.Acknowledgement(func() {
			p.handleMobEffect(pk)
		})
	case *packet.UpdateAbilities:
		p.Acknowledgement(func() {
			for _, l := range pk.AbilityData.Layers {
				p.miMu.Lock()
				p.mInfo.Flying = utils.HasFlag(uint64(l.Values), protocol.AbilityFlying)
				p.mInfo.CanFly = utils.HasFlag(uint64(l.Values), protocol.AbilityMayFly)
				p.miMu.Unlock()
			}
		})
	case *packet.Respawn:
		if pk.EntityRuntimeID != p.rid || pk.State != packet.RespawnStateReadyToSpawn {
			return false
		}

		p.Acknowledgement(func() {
			p.mInfo.ServerPosition = pk.Position.Add(mgl32.Vec3{0, 1.62})
			p.dead = false
			p.respawned = true
		})
	case *packet.Disconnect:
		p.Close()
	}
	return false
}

// handlePlayerAuthInput processes the input packet sent by the client to the server. This also updates some of the movement states such as
// if the player is sprinting, jumping, or in a loaded chunk.
func (p *Player) handlePlayerAuthInput(pk *packet.PlayerAuthInput) {
	p.miMu.Lock()
	defer p.miMu.Unlock()

	p.inputMode = pk.InputMode
	p.Move(pk)

	if p.movementMode == utils.ModeClientAuthoritative {
		p.setMovementToClient()
		return
	}

	defer func() {
		p.mInfo.LastUsedInput = pk
	}()

	p.inLoadedChunk = p.ChunkExists(protocol.ChunkPos{
		int32(math32.Floor(p.mInfo.ServerPosition[0])) >> 4,
		int32(math32.Floor(p.mInfo.ServerPosition[2])) >> 4,
	})

	if p.inLoadedChunk {
		p.inLoadedChunkTicks++
	} else {
		p.inLoadedChunkTicks = 0
	}

	p.mInfo.ForwardImpulse = pk.MoveVector.Y() * 0.98
	p.mInfo.LeftImpulse = pk.MoveVector.X() * 0.98

	if utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting) {
		p.mInfo.Sprinting = true
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting) {
		p.mInfo.Sprinting = false
	}

	if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) {
		p.mInfo.Sneaking = true
	} else if utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
		p.mInfo.Sneaking = false
	}

	p.mInfo.Jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)
	p.mInfo.SprintDown = utils.HasFlag(pk.InputData, packet.InputFlagSprintDown)
	p.mInfo.SneakDown = utils.HasFlag(pk.InputData, packet.InputFlagSneakDown) || utils.HasFlag(pk.InputData, packet.InputFlagSneakToggleDown)
	p.mInfo.JumpDown = utils.HasFlag(pk.InputData, packet.InputFlagJumpDown)
	p.mInfo.InVoid = p.Position().Y() < -128

	p.mInfo.JumpVelocity = game.DefaultJumpMotion
	p.mInfo.Gravity = game.NormalGravity

	p.tickEffects()

	if utils.HasFlag(pk.InputData, packet.InputFlagPerformBlockActions) {
		for _, action := range pk.BlockActions {
			if action.Action != protocol.PlayerActionPredictDestroyBlock {
				continue
			}

			pos := utils.BlockToCubePos(action.BlockPos).Side(cube.Face(action.Face))
			p.makeChunkCopy(GetChunkPos(int32(pos[0]), int32(pos[2])))

			b, _ := world.BlockByRuntimeID(air)
			p.SetBlock(pos, b)
		}
	}

	if p.updateMovementState() {
		p.validateMovement()
	}

	if p.movementMode == utils.ModeFullAuthoritative {
		pk.Position = p.mInfo.ServerPosition.Add(mgl32.Vec3{0, 1.62})
	}

	p.mInfo.Teleporting = false
}

func (p *Player) handleLevelChunk(pk *packet.LevelChunk) {
	if pk.SubChunkCount == protocol.SubChunkRequestModeLimited || pk.SubChunkCount == protocol.SubChunkRequestModeLimitless {
		return
	}

	if p.movementMode == utils.ModeClientAuthoritative {
		return
	}

	c, err := chunk.NetworkDecode(air, pk.RawPayload, int(pk.SubChunkCount), world.Overworld.Range())
	if err != nil {
		c = chunk.New(air, world.Overworld.Range())
	}
	c.Compact()

	switch p.movementMode {
	case utils.ModeSemiAuthoritative:
		p.Acknowledgement(func() {
			tryAddChunkToCache(p, pk.Position, c)
		})
	case utils.ModeFullAuthoritative:
		tryAddChunkToCache(p, pk.Position, c)
	}
	return
}

func (p *Player) handleSubChunk(pk *packet.SubChunk) {
	for _, entry := range pk.SubChunkEntries {
		if entry.Result != protocol.SubChunkResultSuccess && entry.Result != protocol.SubChunkResultSuccessAllAir {
			continue
		}

		chunkPos := protocol.ChunkPos{
			pk.Position[0] + int32(entry.Offset[0]),
			pk.Position[2] + int32(entry.Offset[2]),
		}

		c := chunk.New(air, dimensionFromNetworkID(pk.Dimension).Range())

		if entry.Result == protocol.SubChunkResultSuccessAllAir {
			var index byte
			sub, err := chunk_subChunkDecode(bytes.NewBuffer(entry.RawPayload), c, &index, chunk.NetworkEncoding)
			if err != nil {
				panic(err)
			}

			c.Sub()[index] = sub
		}

		if !tryAddChunkToCache(p, chunkPos, c) {
			panic("unable to add wtf???")
		}
	}
}

func (p *Player) handleBlockPlace(t *protocol.UseItemTransactionData) bool {
	i, ok := world.ItemByRuntimeID(t.HeldItem.Stack.NetworkID, int16(t.HeldItem.Stack.MetadataValue))
	if !ok {
		return false
	}

	b, ok := i.(world.Block)
	if !ok {
		return false
	}

	replacePos := utils.BlockToCubePos(t.BlockPosition)
	fb := p.Block(replacePos)

	if replaceable, ok := fb.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
		replacePos = replacePos.Side(cube.Face(t.BlockFace))
	}

	bx := b.Model().BBox(df_cube.Pos(replacePos), nil)
	boxes := make([]cube.BBox, 0)
	for _, b := range bx {
		boxes = append(boxes, game.DFBoxToCubeBox(b))
	}

	bb := p.AABB().Translate(t.Position)
	if utils.BoxesIntersect(bb, boxes, replacePos.Vec3()) {
		return false
	}

	p.makeChunkCopy(GetChunkPos(int32(replacePos[0]), int32(replacePos[2])))

	// Set the block in the world
	p.SetBlock(replacePos, b)
	return false
}

func (p *Player) handleMobEffect(pk *packet.MobEffect) {
	switch pk.Operation {
	case packet.MobEffectAdd, packet.MobEffectModify:
		t, ok := effect.ByID(int(pk.EffectType))
		if !ok {
			return
		}

		e, ok := t.(effect.LastingType)
		if !ok {
			return
		}

		eff := effect.New(e, int(pk.Amplifier)+1, time.Duration(pk.Duration*50)*time.Millisecond)
		p.SetEffect(pk.EffectType, eff)
	case packet.MobEffectRemove:
		p.RemoveEffect(pk.EffectType)
	}
}

func (p *Player) handleSetActorData(pk *packet.SetActorData) {
	isPlayer := pk.EntityRuntimeID == p.rid
	width, widthExists := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]
	height, heightExists := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]

	e, ok := p.SearchEntity(pk.EntityRuntimeID)
	if isPlayer {
		e = p.Entity()
		ok = true
	}

	if !ok {
		return
	}

	if widthExists {
		e.SetAABB(game.AABBFromDimensions(width.(float32), e.AABB().Height()))
	}
	if heightExists {
		e.SetAABB(game.AABBFromDimensions(e.AABB().Width(), height.(float32)))
	}

	if !isPlayer {
		return
	}

	f, ok := pk.EntityMetadata[entity.DataKeyFlags]
	if !ok {
		return
	}
	flags := f.(int64)

	p.miMu.Lock()
	p.mInfo.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, flags)
	p.mInfo.Sprinting = utils.HasDataFlag(entity.DataFlagSprinting, flags)
	p.mInfo.Sneaking = utils.HasDataFlag(entity.DataFlagSneaking, flags)
	p.miMu.Unlock()
}

// dimensionFromNetworkID returns a world.Dimension from the network id.
func dimensionFromNetworkID(id int32) world.Dimension {
	if id == 1 {
		return world.Nether
	}
	if id == 2 {
		return world.End
	}
	return world.Overworld
}

// noinspection ALL
//
//go:linkname chunk_subChunkDecode github.com/df-mc/dragonfly/server/world/chunk.decodeSubChunk
func chunk_subChunkDecode(buf *bytes.Buffer, c *chunk.Chunk, index *byte, e chunk.Encoding) (*chunk.SubChunk, error)
