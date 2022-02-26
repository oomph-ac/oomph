package player

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/justtaldevelops/oomph/entity"
	"github.com/justtaldevelops/oomph/game"
	"github.com/justtaldevelops/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ClientProcess processes the given packet from the client.
func (p *Player) ClientProcess(pk packet.Packet) bool {
	p.clicking.Store(false)

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
		p.clientTick++
		p.Move(pk)

		if utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting) || utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting) {
			p.sprinting.Toggle()
		} else if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) || utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
			p.sneaking.Toggle()
		}

		p.jumping.Store(utils.HasFlag(pk.InputData, packet.InputFlagStartJumping))
		p.tickEntityLocations()
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			p.Click()
		}
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			p.Click()
		}
	case *packet.AdventureSettings:
		p.flying.Store(utils.HasFlag(uint64(pk.Flags), packet.AdventureFlagFlying))
	case *packet.Respawn:
		if pk.EntityRuntimeID == p.rid && pk.State == packet.RespawnStateClientReadyToSpawn {
			p.dead.Store(false)
		}
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
		c.Process(p, pk)
	}
	return false
}

// ServerProcess processes the given packet from the server.
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
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				p.Teleport(pk.Position)
				if utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport) {
					p.teleporting.Store(true)
				}
			})
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position))
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				p.Teleport(pk.Position)
				if pk.Mode == packet.MoveModeTeleport {
					p.teleporting.Store(true)
				}
			})
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position))
	case *packet.LevelChunk:
		p.Acknowledgement(func() {
			p.ready.Store(true)
			p.LoadRawChunk(world.ChunkPos{pk.Position.X(), pk.Position.Z()}, pk.RawPayload, pk.SubChunkCount)
		})
	case *packet.UpdateBlock:
		b, ok := world.BlockByRuntimeID(pk.NewBlockRuntimeID)
		if ok {
			p.Acknowledgement(func() {
				p.SetBlock(cube.Pos{int(pk.Position.X()), int(pk.Position.Y()), int(pk.Position.Z())}, b)
			})
		}
	case *packet.SetActorData:
		p.Acknowledgement(func() {
			width, widthExists := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]
			height, heightExists := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]
			if e, ok := p.SearchEntity(pk.EntityRuntimeID); ok && widthExists && heightExists {
				e.SetAABB(game.AABBFromDimensions(float64(width.(float32)), float64(height.(float32))))
			}

			if f, ok := pk.EntityMetadata[entity.DataKeyFlags]; pk.EntityRuntimeID == p.rid && ok {
				p.immobile.Store(utils.HasDataFlag(entity.DataFlagImmobile, f.(int64)))
			}
		})
	case *packet.SetPlayerGameType:
		p.Acknowledgement(func() {
			p.gameMode.Store(pk.GameType)
		})
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
						p.dead.Store(true)
					}
				}
			})
		}
	}
	return false
}
