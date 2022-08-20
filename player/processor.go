package player

import (
	"bytes"
	"time"
	_ "unsafe"

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
		}, false)
	case *packet.NetworkStackLatency:
		p.ackMu.Lock()
		cancel = p.acks.Handle(pk.Timestamp)
		p.ackMu.Unlock()
	case *packet.PlayerAuthInput:
		p.clientTick.Inc()
		if pk.Tick < p.ClientFrame() {
			p.Disconnect("AC Error: Invalid frame recieved in ticked input")
		}
		p.clientFrame.Store(pk.Tick)

		p.processInput(pk)
		p.acks.HasTicked = true
		p.cleanChunks()

		p.hasValidatedCombat = false
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			p.Click()
		}
	case *packet.InventoryTransaction:
		if hit, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			cancel = !p.validateCombat(hit)
			p.Click()
		} else if t, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok && t.ActionType == protocol.UseItemActionClickBlock {
			pos := cube.Pos{int(t.BlockPosition.X()), int(t.BlockPosition.Y()), int(t.BlockPosition.Z())}
			block, ok := world.BlockByRuntimeID(t.BlockRuntimeID)
			if !ok {
				// The block somehow doesn't exist, so nothing can be done.
				return false
			}

			boxes := block.Model().BBox(pos, nil)
			for _, box := range boxes {
				if box.Translate(pos.Vec3()).IntersectsWith(p.AABB().Translate(p.mInfo.ServerPosition)) {
					// The block would intersect with our AABB, so a block would not be placed.
					return false
				}
			}

			// Set the block in the world
			p.SetBlock(pos.Side(cube.Face(t.BlockFace)), block)
		}
	case *packet.AdventureSettings:
		p.mInfo.Flying = utils.HasFlag(uint64(pk.Flags), packet.AdventureFlagFlying)
		p.mInfo.CanNoClip = utils.HasFlag(uint64(pk.Flags), packet.AdventureFlagNoClip)
	case *packet.Respawn:
		if pk.EntityRuntimeID == p.rid && pk.State == packet.RespawnStateClientReadyToSpawn {
			p.dead = false
		}
	case *packet.Text:
		if p.serverConn != nil {
			// Strip the XUID to prevent certain server software from flagging the message as spam.
			pk.XUID = ""
		}
	case *packet.Login:
		if os, ok := map[string]protocol.DeviceOS{
			"1739947436": protocol.DeviceAndroid,
			"1810924247": protocol.DeviceIOS,
			"1944307183": protocol.DeviceFireOS,
			"896928775":  protocol.DeviceWin10,
			"2044456598": protocol.DeviceOrbis,
			"2047319603": protocol.DeviceNX,
			"1828326430": protocol.DeviceXBOX,
			"1916611344": protocol.DeviceWP,
		}[p.IdentityData().TitleID]; ok {
			p.gamePlatform = os
		} else {
			p.gamePlatform = protocol.DeviceLinux
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
		}, false)
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
		}, false)
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID == p.rid {
			teleport := utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport)
			if teleport {
				p.Acknowledgement(func() {
					p.Teleport(pk.Position, teleport)
				}, false)
			}
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position), utils.HasFlag(uint64(pk.Flags), packet.MoveFlagOnGround))
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.rid {
			teleport := pk.Mode == packet.MoveModeTeleport
			if !teleport {
				return false
			}

			if pk.Tick != 0 {
				p.Teleport(pk.Position, teleport)
				return false
			}

			p.Acknowledgement(func() {
				p.Teleport(pk.Position, teleport)
			}, false)
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position), pk.OnGround)
	case *packet.SetActorData:
		p.Acknowledgement(func() {
			width, widthExists := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]
			height, heightExists := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]
			if e, ok := p.SearchEntity(pk.EntityRuntimeID); ok {
				if widthExists {
					width := game.Round(float64(width.(float32)), 5)
					e.SetAABB(game.AABBFromDimensions(width, e.AABB().Height()))
				}
				if heightExists {
					height := game.Round(float64(height.(float32)), 5)
					e.SetAABB(game.AABBFromDimensions(e.AABB().Width(), height))
				}
			}

			if f, ok := pk.EntityMetadata[entity.DataKeyFlags]; pk.EntityRuntimeID == p.rid && ok {
				flags := f.(int64)
				p.mInfo.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, flags)
				p.mInfo.Sprinting = utils.HasDataFlag(entity.DataFlagSprinting, flags)
				p.mInfo.Sneaking = utils.HasDataFlag(entity.DataFlagSneaking, flags)
			}
		}, false)
	case *packet.SetPlayerGameType:
		p.Acknowledgement(func() {
			p.gameMode = pk.GameType
		}, false)
	case *packet.RemoveActor:
		if pk.EntityUniqueID != p.uid {
			p.Acknowledgement(func() {
				p.RemoveEntity(uint64(pk.EntityUniqueID))
			}, false)
		}
	case *packet.UpdateAttributes:
		pk.Tick = 0 // prevent any rewind from being done to prevent shit-fuckery with incorrect movement
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				for _, a := range pk.Attributes {
					if a.Name == "minecraft:health" && a.Value <= 0 {
						p.dead = true
					} else if a.Name == "minecraft:movement" {
						p.mInfo.Speed = float64(a.Value)
					}
				}
			}, false)
		}
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.rid {
			return false
		}

		velocity := game.Vec32To64(pk.Velocity)

		// If the player is behind by more than 5 ticks (250ms), then instantly set the KB
		// of the player instead of waiting for an acknowledgement. This will ensure that players
		// with very high latency do not get a significant advantage due to them receiving knockback late.
		if int64(p.serverTick.Load())-int64(p.clientTick.Load()) > 5 {
			p.mInfo.UpdateServerSentVelocity(velocity)
			return false
		}

		// Send an acknowledgement to the player to get the client tick where the player will apply KB and verify that the client
		// does take knockback when it recieves it.
		p.Acknowledgement(func() {
			p.mInfo.UpdateServerSentVelocity(velocity)
		}, false)
	case *packet.LevelChunk:
		if !p.mPredictions {
			return false
		}

		c, err := chunk.NetworkDecode(air, pk.RawPayload, int(pk.SubChunkCount), world.Overworld.Range())
		if err != nil {
			c = chunk.New(air, world.Overworld.Range())
		}

		c.Compact()
		p.LoadChunk(pk.Position, c)
		p.ready = true
	case *packet.SubChunk:
		if !p.mPredictions {
			return false
		}

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
	case *packet.ChunkRadiusUpdated:
		p.chunkRadius = int(pk.ChunkRadius) + 4
	case *packet.UpdateBlock:
		b, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if ok {
			p.SetBlock(cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}, b)
		}
	case *packet.MobEffect:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				switch pk.Operation {
				case packet.MobEffectAdd, packet.MobEffectModify:
					if t, ok := effect.ByID(int(pk.EffectType)); ok {
						if t, ok := t.(effect.LastingType); ok {
							eff := effect.New(t, int(pk.Amplifier)+1, time.Duration(pk.Duration*50)*time.Millisecond)
							p.SetEffect(pk.EffectType, eff)
						}
					}
				case packet.MobEffectRemove:
					p.RemoveEffect(pk.EffectType)
				}
			}, false)
		}
	}
	return false
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