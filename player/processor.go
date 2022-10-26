package player

import (
	"bytes"
	"math"
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
			p.ready = true
		})
		p.rid = p.conn.GameData().EntityRuntimeID
	case *packet.NetworkStackLatency:
		p.ackMu.Lock()
		cancel = p.acks.Handle(pk.Timestamp)
		p.ackMu.Unlock()
	case *packet.PlayerAuthInput:
		p.clientTick.Inc()
		p.clientFrame.Store(pk.Tick)

		if p.movementMode == utils.ModeSemiAuthoritative {
			defer func() {
				// After processing movement and letting checks validate movement, set the server's position and movement to the client's.
				p.mInfo.ServerPosition = p.Position()
				p.mInfo.ServerMovement = game.Vec32To64(pk.Delta)
			}()
		}

		p.cleanChunks()
		p.processInput(pk)

		if p.combatMode == utils.ModeFullAuthoritative {
			p.validateCombat()
		} else {
			p.tickEntitiesPos()
		}

		if p.acks != nil {
			p.acks.HasTicked = true
		}

		p.needsCombatValidation = false
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
			defer func() {
				p.lastRightClickData = t
				p.lastRightClickTick = p.ClientFrame()
			}()

			i, ok := world.ItemByRuntimeID(t.HeldItem.Stack.NetworkID, int16(t.HeldItem.Stack.MetadataValue))
			if !ok {
				return false
			}

			b, ok := i.(world.Block)
			if !ok {
				return false
			}

			replacePos := cube.Pos{int(t.BlockPosition.X()), int(t.BlockPosition.Y()), int(t.BlockPosition.Z())}
			fb := p.Block(replacePos)

			if replaceable, ok := fb.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
				replacePos = replacePos.Side(cube.Face(t.BlockFace))
			}

			boxes := b.Model().BBox(replacePos, nil)
			for _, box := range boxes {
				if box.Translate(replacePos.Vec3()).IntersectsWith(p.AABB().Translate(game.Vec32To64(t.Position))) {
					// The block would intersect with our AABB, so a block would not be placed.
					return false
				}
			}

			spam := false

			// This code will detect if the client is sending this packet due to a right click bug where this will be spammed to the server.
			if p.lastRightClickData != nil {
				spam = p.ClientFrame()-p.lastRightClickTick < 2
				spam = spam && p.lastRightClickData.Position == t.Position
				spam = spam && p.lastRightClickData.BlockPosition == t.BlockPosition
				spam = spam && p.lastRightClickData.ClickedPosition == t.ClickedPosition
			}

			if spam {
				// Cancel the sending of this packet if we determine that it's the right click spam bug.
				l := uint32(0)
				if _, ok := fb.(world.Liquid); ok {
					l = 1
				}

				p.conn.WritePacket(&packet.UpdateBlock{
					Position:          protocol.BlockPos{int32(replacePos.X()), int32(replacePos.Y()), int32(replacePos.Z())},
					NewBlockRuntimeID: world.BlockRuntimeID(fb),
					Layer:             l,
					Flags:             packet.BlockUpdatePriority,
				})
				return true
			}

			// Set the block in the world
			p.SetBlock(replacePos, b)
		}
	case *packet.Respawn:
		if pk.EntityRuntimeID != p.rid {
			return false
		}

		p.dead = pk.State != packet.RespawnStateClientReadyToSpawn
	case *packet.Text:
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
		if pk.Tick != 0 {
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

		p.Acknowledgement(func() {
			width, widthExists := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]
			height, heightExists := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]

			if e, ok := p.SearchEntity(pk.EntityRuntimeID); ok {
				if widthExists {
					e.SetAABB(game.AABBFromDimensions(float64(width.(float32)), e.AABB().Height()))
				}

				if heightExists {
					e.SetAABB(game.AABBFromDimensions(e.AABB().Width(), float64(height.(float32))))
				}
			}

			if pk.EntityRuntimeID != p.rid {
				return
			}

			if f, ok := pk.EntityMetadata[entity.DataKeyFlags]; ok {
				flags := f.(int64)
				p.mInfo.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, flags)
				p.mInfo.Sprinting = utils.HasDataFlag(entity.DataFlagSprinting, flags)
				p.mInfo.Sneaking = utils.HasDataFlag(entity.DataFlagSneaking, flags)
			}
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
					p.mInfo.Speed = float64(a.Value)
				}
			}
		})
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.rid {
			return false
		}

		velocity := game.Vec32To64(pk.Velocity)

		// If the player is behind by more than 5 ticks (250ms), then instantly set the KB
		// of the player instead of waiting for an acknowledgement. This will ensure that players
		// with very high latency do not get a significant advantage due to them receiving knockback late.
		if p.movementMode == utils.ModeFullAuthoritative && int64(p.serverTick.Load())-int64(p.clientTick.Load()) >= 5 {
			p.mInfo.UpdateServerSentVelocity(velocity)
			return false
		}

		// Send an acknowledgement to the player to get the client tick where the player will apply KB and verify that the client
		// does take knockback when it recieves it.
		p.Acknowledgement(func() {
			p.mInfo.UpdateServerSentVelocity(velocity)
		})
	case *packet.LevelChunk:
		if p.movementMode == utils.ModeClientAuthoritative {
			return false
		}

		c, err := chunk.NetworkDecode(air, pk.RawPayload, int(pk.SubChunkCount), world.Overworld.Range())
		if err != nil {
			c = chunk.New(air, world.Overworld.Range())
		}
		c.Compact()

		p.Acknowledgement(func() {
			pos := protocol.ChunkPos{int32(math.Floor(p.mInfo.ServerPosition[0])) >> 4, int32(math.Floor(p.mInfo.ServerPosition[2])) >> 4}
			if pos == pk.Position && p.ChunkExists(pos) {
				p.inLoadedChunkTicks = 0
			}
			p.LoadChunk(pk.Position, c)
		})
	case *packet.SubChunk:
		if p.movementMode == utils.ModeClientAuthoritative {
			return false
		}

		p.Acknowledgement(func() {
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
		})
	case *packet.ChunkRadiusUpdated:
		p.chunkRadius = int(pk.ChunkRadius) + 4
	case *packet.UpdateBlock:
		if p.movementMode == utils.ModeClientAuthoritative {
			return false
		}

		b, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if !ok {
			return false
		}

		if p.movementMode == utils.ModeFullAuthoritative {
			p.SetBlock(cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}, b)
			return false
		}

		p.Acknowledgement(func() {
			p.SetBlock(cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}, b)
		})
	case *packet.MobEffect:
		if pk.EntityRuntimeID != p.rid {
			return false
		}

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
		})
	case *packet.UpdateAbilities:
		p.Acknowledgement(func() {
			for _, l := range pk.AbilityData.Layers {
				p.mInfo.Flying = utils.HasFlag(uint64(l.Values), protocol.AbilityFlying)
				p.mInfo.CanFly = utils.HasFlag(uint64(l.Values), protocol.AbilityMayFly)
			}
		})
	case *packet.ChangeDimension:
		p.ready = false
		p.Acknowledgement(func() {
			p.ready = true
		})
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
