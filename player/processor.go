package player

import (
	"time"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ClientProcess processes a given packet from the client.
func (p *Player) ClientProcess(pk packet.Packet) bool {
	cancel := false
	p.clicking = false

	switch pk := pk.(type) {
	case *packet.NetworkStackLatency:
		p.ackMu.Lock()
		call, ok := p.acknowledgements[pk.Timestamp]
		if ok {
			delete(p.acknowledgements, pk.Timestamp)
			call()
			cancel = true
		}
		p.ackMu.Unlock()
	case *packet.PlayerAuthInput:
		p.clientTick.Inc()
		p.clientFrame.Store(pk.Tick)

		p.processInput(pk)
		p.cleanChunks()
		p.tickEntityLocations()
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			p.Click()
		}
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			if !p.validateCombat(pk) {
				cancel = true
			}
			p.Click()
		} else if t, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok && t.ActionType == protocol.UseItemActionClickBlock {
			pos := cube.Pos{int(t.BlockPosition.X()), int(t.BlockPosition.Y()), int(t.BlockPosition.Z())}
			block, ok := world.BlockByRuntimeID(t.BlockRuntimeID)
			if !ok {
				// Block somehow doesn't exist, so do nothing.
				return false
			}

			boxes := block.Model().BBox(pos, nil)
			for _, box := range boxes {
				if box.Translate(pos.Vec3()).IntersectsWith(p.AABB().Translate(p.Position())) {
					// Intersects with our AABB, so do nothing.
					return false
				}
			}

			// Set the block in the world
			p.SetBlock(pos, block)
		}
	case *packet.AdventureSettings:
		p.MovementInfo().Flying = utils.HasFlag(uint64(pk.Flags), packet.AdventureFlagFlying)
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
			if teleport {
				p.Acknowledgement(func() {
					p.Teleport(pk.Position, teleport)
				}, false)
			}
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position), pk.OnGround)
	case *packet.SetActorData:
		mInfo := p.MovementInfo()
		pk.Tick = p.ClientFrame()

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
			mInfo.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, flags)
			mInfo.Sprinting = utils.HasDataFlag(entity.DataFlagSprinting, flags)
			mInfo.Sneaking = utils.HasDataFlag(entity.DataFlagSneaking, flags)
		}
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
		pk.Tick = p.ClientFrame()
		if pk.EntityRuntimeID == p.rid {
			for _, a := range pk.Attributes {
				if a.Name == "minecraft:health" && a.Value <= 0 {
					p.dead = true
				} else if a.Name == "minecraft:movement" {
					p.mInfo.Speed = float64(a.Value)
				}
			}
		}
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID == p.rid {
			velocity := mgl64.Vec3{
				float64(pk.Velocity[0]),
				float64(pk.Velocity[1]),
				float64(pk.Velocity[2]),
			}

			// Send an acknowledgement to the player to get the client tick where the player will apply KB and verify that the client
			// does take knockback when it recieves it.
			/* p.Acknowledgement(func() {
				p.mInfo.UpdateServerSentVelocity(velocity)
			}, false) */

			// The server movement is updated to the knockback sent by this packet. Regardless of wether
			// the client has recieved knockback - the server's movement should be the knockback sent by the server.
			p.mInfo.UpdateServerSentVelocity(velocity)
		} else if e, ok := p.SearchEntity(pk.EntityRuntimeID); ok && !e.Player() {
			p.queuedEntityMotionInterpolations[pk.EntityRuntimeID] = game.Vec32To64(pk.Velocity)
		}
	case *packet.LevelChunk:
		go func() {
			a, _ := chunk.StateToRuntimeID("minecraft:air", nil)
			c, err := chunk.NetworkDecode(a, pk.RawPayload, int(pk.SubChunkCount), world.Overworld.Range())
			if err != nil {
				panic(err)
			}

			c.Compact()
			p.LoadChunk(pk.Position, c)
			p.ready = true
		}()
	case *packet.ChunkRadiusUpdated:
		p.chunkRadius = int(pk.ChunkRadius) + 4
	case *packet.UpdateBlock:
		b, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if ok {
			p.Acknowledgement(func() {
				p.SetBlock(cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}, b)
			}, false)
		}
	case *packet.MobEffect:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				switch pk.Operation {
				case packet.MobEffectAdd, packet.MobEffectModify:
					if t, ok := effect.ByID(int(pk.EffectType)); ok {
						if t, ok := t.(effect.LastingType); ok {
							eff := effect.New(t, int(pk.Amplifier+1), time.Duration(pk.Duration*50)*time.Millisecond)
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
