package player

import (
	"bytes"
	"strings"
	"time"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
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

	if p.closed {
		return false
	}

	switch pk := pk.(type) {
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
		})
		p.rid = p.conn.GameData().EntityRuntimeID
	case *packet.NetworkStackLatency:
		cancel = p.Acknowledgements().Handle(pk.Timestamp, p.ClientData().DeviceOS == protocol.DeviceOrbis)
	case *packet.PlayerAuthInput:
		p.clientTick.Inc()
		p.clientFrame.Store(pk.Tick)

		p.cleanChunks()
		p.processInput(pk)

		if p.combatMode == utils.ModeFullAuthoritative {
			p.validateCombat()
		} else {
			p.tickEntitiesPos()
		}

		if acks := p.Acknowledgements(); acks != nil {
			acks.HasTicked = true
		}
		p.needsCombatValidation = false

		defer p.SetRespawned(false)
		if p.movementMode == utils.ModeSemiAuthoritative {
			defer p.setMovementToClient(pk.Delta)
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
			case "server_kb":
			case "server_knockback":
				p.debugger.ServerKnockback = b
			default:
				p.SendOomphDebug("Unknown debug mode: "+cmd[1], packet.TextTypeChat)
			}
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
func (p *Player) ServerProcess(pk packet.Packet) bool {
	if p.closed {
		return false
	}

	switch pk := pk.(type) {
	case *packet.AddPlayer:
		if pk.EntityRuntimeID == p.rid {
			// We are the player.
			return false
		}

		p.Acknowledgement(func() {
			p.AddEntity(pk.EntityRuntimeID, entity.NewEntity(
				game.Vec32To64(pk.Position),
				game.Vec32To64(pk.Velocity),
				game.Vec32To64(mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw}),
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
				game.Vec32To64(pk.Position),
				game.Vec32To64(pk.Velocity),
				game.Vec32To64(mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw}),
				false,
			))
		})
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.rid {
			p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position), utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport), utils.HasFlag(uint64(pk.Flags), packet.MoveFlagOnGround))
			return false
		}

		if !utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
			return false
		}

		p.Acknowledgement(func() {
			p.Teleport(pk.Position, true)
		})
	case *packet.MovePlayer:
		if pk.EntityRuntimeID != p.rid {
			p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position), pk.Mode == packet.MoveModeTeleport, pk.OnGround)
			return false
		}

		if pk.Mode != packet.MoveModeTeleport {
			return false
		}

		// If rewind is being applied with this packet, teleport the player instantly on the server
		// instead of waiting for the client to acknowledge the packet.
		if p.movementMode == utils.ModeFullAuthoritative {
			pk.Tick = p.ClientFrame()
			p.Teleport(pk.Position, true)
			return false
		}

		p.Acknowledgement(func() {
			p.Teleport(pk.Position, true)
		})
	case *packet.SetActorData:
		pk.Tick = 0 // prevent rewind from happening

		if pk.EntityRuntimeID == p.rid {
			p.lastSentActorData = pk
		}

		if p.movementMode == utils.ModeFullAuthoritative && p.TickLatency() >= NetworkLatencyCutoff {
			pk.Tick = p.ClientFrame()
			p.handleSetActorData(pk)
			return false
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

		p.lastSentAttributes = pk
		p.Acknowledgement(func() {
			for _, a := range pk.Attributes {
				if a.Name == "minecraft:health" && a.Value <= 0 {
					p.isSyncedWithServer = false
					p.dead = true
				} else if a.Name == "minecraft:movement" {
					p.miMu.Lock()
					p.mInfo.Speed = float64(a.Value)
					p.miMu.Unlock()
				}
			}
		})
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.rid {
			return false
		}
		v := game.Vec32To64(pk.Velocity)

		// If the player is behind by more than 6 ticks (300ms), then instantly set the KB
		// of the player instead of waiting for an acknowledgement. This will ensure that players
		// with very high latency do not get a significant advantage due to them receiving knockback late.
		if p.movementMode == utils.ModeFullAuthoritative && (p.TickLatency() >= NetworkLatencyCutoff || p.debugger.ServerKnockback) {
			p.UpdateServerVelocity(v)
			return false
		}

		// Send an acknowledgement to the player to get the client tick where the player will apply KB and verify that the client
		// does take knockback when it recieves it.
		p.Acknowledgement(func() {
			p.UpdateServerVelocity(v)
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
		p.Acknowledgement(func() {
			p.chunkRadius = int(pk.ChunkRadius) + 4
		})
	case *packet.UpdateBlock:
		if p.movementMode == utils.ModeClientAuthoritative {
			return false
		}

		b, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			return false
		}

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
			p.dead = false
			p.respawned = true
		})
	}
	return false
}

func (p *Player) handleLevelChunk(pk *packet.LevelChunk) {
	p.ready = true
	c, err := chunk.NetworkDecode(air, pk.RawPayload, int(pk.SubChunkCount), world.Overworld.Range())
	if err != nil {
		c = chunk.New(air, world.Overworld.Range())
	}
	c.Compact()

	switch p.movementMode {
	case utils.ModeSemiAuthoritative:
		p.Acknowledgement(func() {
			p.LoadChunk(pk.Position, c)
		})
	case utils.ModeFullAuthoritative:
		p.LoadChunk(pk.Position, c)
	}
}

func (p *Player) handleSubChunk(pk *packet.SubChunk) {
	p.ready = true
	for _, entry := range pk.SubChunkEntries {
		if entry.Result != protocol.SubChunkResultSuccess {
			continue
		}

		chunkPos := protocol.ChunkPos{
			pk.Position[0] + int32(entry.Offset[0]),
			pk.Position[2] + int32(entry.Offset[2]),
		}

		c, ok := p.Chunk(chunkPos)
		if !ok {
			p.chkMu.Lock()
			c = chunk.New(air, dimensionFromNetworkID(pk.Dimension).Range())
			p.chunks[chunkPos] = c
			p.chkMu.Unlock()
		} else {
			c.Unlock()
		}

		var index byte
		sub, err := chunk_subChunkDecode(bytes.NewBuffer(entry.RawPayload), c, &index, chunk.NetworkEncoding)
		if err != nil {
			panic(err)
		}

		c.Sub()[index] = sub
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

	boxes := b.Model().BBox(replacePos, nil)
	bb := p.AABB().Translate(game.Vec32To64(t.Position))
	if utils.BoxesIntersect(bb, boxes, replacePos.Vec3()) {
		return false
	}

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
		e.SetAABB(game.AABBFromDimensions(float64(width.(float32)), e.AABB().Height()))
	}
	if heightExists {
		e.SetAABB(game.AABBFromDimensions(e.AABB().Width(), float64(height.(float32))))
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
