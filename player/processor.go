package player

import (
	"math"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block/cube"
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
	p.clicking = false

	switch pk := pk.(type) {
	case *packet.NetworkStackLatency:
		p.ackMu.Lock()
		call, ok := p.acknowledgements[pk.Timestamp]
		if ok {
			delete(p.acknowledgements, pk.Timestamp)
			p.ackMu.Unlock()

			call()
			return true
		}
		p.ackMu.Unlock()
	case *packet.PlayerAuthInput:
		p.clientTick.Inc()
		p.clientFrame.Store(pk.Tick)

		p.wMu.Lock()
		p.inLoadedChunk = world_chunkExists(p.world, p.Position()) // Not being in a loaded chunk can cause issues with movement predictions - especially when collision checks are done
		p.wMu.Unlock()

		p.Move(pk)

		p.mInfo.MoveForward = float64(pk.MoveVector.Y()) * 0.98
		p.mInfo.MoveStrafe = float64(pk.MoveVector.X()) * 0.98

		if utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting) {
			p.mInfo.Sprinting = true
		}
		if utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting) {
			p.mInfo.Sprinting = false
		}
		if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) {
			p.mInfo.Sneaking = true
		}
		if utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
			p.mInfo.Sneaking = false
		}
		p.mInfo.Jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)
		p.mInfo.InVoid = p.Position().Y() < -35

		p.mInfo.JumpVelocity = game.DefaultJumpMotion
		p.mInfo.Speed = game.NormalMovementSpeed
		p.mInfo.Gravity = game.NormalGravity

		if p.mInfo.Sprinting {
			p.mInfo.Speed *= 1.3
		}

		p.updateMovementState()
		p.tickEntityLocations()
		p.WorldLoader().Move(p.mInfo.ServerPredictedPosition)

		pk.Position = game.Vec64To32(p.mInfo.ServerPredictedPosition.Add(mgl64.Vec3{0, 1.62}))
		p.mInfo.Teleporting = false
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			p.Click()
		}
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			p.Click()
		} else if t, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok && t.ActionType == protocol.UseItemActionClickBlock {
			pos := cube.Pos{int(t.BlockPosition.X()), int(t.BlockPosition.Y()), int(t.BlockPosition.Z())}
			block, ok := world.BlockByRuntimeID(t.BlockRuntimeID)
			if !ok {
				// Block somehow doesn't exist, so do nothing.
				return false
			}

			w := p.World()
			boxes := block.Model().BBox(pos, w)
			for _, box := range boxes {
				if box.Translate(pos.Vec3()).IntersectsWith(p.AABB().Translate(p.Position())) {
					// Intersects with our AABB, so do nothing.
					return false
				}
			}

			// Set the block in the world
			w.SetBlock(pos, block, nil)
		}
	case *packet.AdventureSettings:
		p.mInfo.Flying = utils.HasFlag(uint64(pk.Flags), packet.AdventureFlagFlying)
	case *packet.Respawn:
		if pk.EntityRuntimeID == p.rid && pk.State == packet.RespawnStateClientReadyToSpawn {
			p.dead = false
		}
	case *packet.Text:
		if p.serverConn != nil {
			// Strip the XUID to prevent certain server software from flagging the message as spam.
			pk.XUID = ""
		}
	case *packet.ChunkRadiusUpdated:
		p.WorldLoader().ChangeRadius(int(pk.ChunkRadius))
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
		c.Process(p, pk)
	}
	return false
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
			p.Acknowledgement(func() {
				teleport := utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport)
				if teleport {
					p.Teleport(pk.Position, teleport)
				}
			}, false)
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position), utils.HasFlag(uint64(pk.Flags), packet.MoveFlagOnGround))
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				teleport := pk.Mode == packet.MoveModeTeleport
				if teleport {
					p.Teleport(pk.Position, teleport)
				}
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
				p.mInfo.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, f.(int64))
			}
		}, false)
	case *packet.SubChunk:
		p.Acknowledgement(func() {
			p.ready = true
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
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				for _, a := range pk.Attributes {
					if a.Name == "minecraft:health" && a.Value <= 0 {
						p.dead = true
					}
				}
			}, false)
		}
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				p.mInfo.UpdateServerSentVelocity(mgl64.Vec3{
					float64(pk.Velocity[0]),
					float64(pk.Velocity[1]),
					float64(pk.Velocity[2]),
				})

			}, false)
		} else if e, ok := p.SearchEntity(pk.EntityRuntimeID); ok && !e.Player() {
			p.queuedEntityMotionInterpolations[pk.EntityRuntimeID] = game.Vec32To64(pk.Velocity)
		}
	case *packet.LevelChunk:
		p.Acknowledgement(func() {
			go func() {
				p.wMu.Lock()
				a, _ := chunk.StateToRuntimeID("minecraft:air", nil)
				c, err := chunk.NetworkDecode(a, pk.RawPayload, int(pk.SubChunkCount), p.world.Range())
				if err != nil {
					p.log.Errorf("failed to parse chunk at %v: %v", pk.Position, err)
					return
				}

				world_setChunk(p.world, world.ChunkPos(pk.Position), c, nil)
				p.wMu.Unlock()
				p.ready = true
			}()
		}, false)
	case *packet.UpdateBlock:
		b, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if ok {
			p.Acknowledgement(func() {
				p.World().SetBlock(cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}, b, nil)
			}, false)
		}
	}
	return false
}

// This function checks if a chunk exists in the world's cache
func world_chunkExists(w *world.World, pos mgl64.Vec3) bool {
	c, ok := world_chunkFromCache(w, world.ChunkPos{
		int32(math.Floor(pos[0])) >> 4,
		int32(math.Floor(pos[2])) >> 4,
	})
	if ok {
		c.Unlock()
	}
	return ok
}

//go:linkname world_setChunk github.com/df-mc/dragonfly/server/world.(*World).setChunk
//noinspection ALL
func world_setChunk(w *world.World, pos world.ChunkPos, c *chunk.Chunk, e map[cube.Pos]world.Block)

//go:linkname world_chunkFromCache github.com/df-mc/dragonfly/server/world.(*World).chunkFromCache
//noinspection ALL
func world_chunkFromCache(w *world.World, pos world.ChunkPos) (*chunkData, bool)

type chunkData struct {
	*chunk.Chunk
	e        map[cube.Pos]world.Block
	v        []world.Viewer
	l        []*world.Loader
	entities []world.Entity
}
