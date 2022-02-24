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
func (p *Player) ClientProcess(pk packet.Packet) {
	p.Session().SetFlag(false, session.FlagClicking)

	switch pk := pk.(type) {
	case *packet.NetworkStackLatency:
		p.ackMu.Lock()
		call, ok := p.acknowledgements[pk.Timestamp]
		if ok {
			call()
			delete(p.acknowledgements, pk.Timestamp)
		}
		p.ackMu.Unlock()
	case *packet.PlayerAuthInput:
		p.clientTick++
		p.Move(pk)

		s := p.Session()

		for inputFlags, sessionFlag := range session.InputFlagMap {
			if utils.HasFlag(pk.InputData, inputFlags[0]) {
				s.SetFlag(true, sessionFlag)
			} else if utils.HasFlag(pk.InputData, inputFlags[1]) {
				s.SetFlag(false, sessionFlag)
			}
		}

		s.SetFlag(utils.HasFlag(pk.InputData, packet.InputFlagStartJumping), session.FlagJumping)

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
			// Strip the XUID to prevent certain server software from flagging the message as spam.
			pk.XUID = ""
		}
	}

	// Run all registered checks.
	p.checkMu.Lock()
	for _, c := range p.checks {
		c.Process(p, pk)
	}
	p.checkMu.Unlock()
}

// ServerProcess processes the given packet from the server.
func (p *Player) ServerProcess(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.AddPlayer:
		if pk.EntityRuntimeID == p.rid {
			// We are the player.
			return
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
			return
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
					p.Session().ASetFlag(true, session.FlagTeleporting)
				}
			})
			return
		}

		p.MoveActor(pk.EntityRuntimeID, game.Vec32To64(pk.Position))
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				p.Teleport(pk.Position)
				if pk.Mode == packet.MoveModeTeleport {
					p.Session().SetFlag(true, session.FlagTeleporting)
				}
			})
			return
		}

		p.MoveActor(pk.EntityRuntimeID, game.Vec32To64(pk.Position))
	case *packet.LevelChunk:
		p.Acknowledgement(func() {
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
				p.Session().SetFlag(hasFlag(entity.DataFlagImmobile, f.(int64)), session.FlagImmobile)
			}
		})
	case *packet.StartGame: // TODO: Change this, as it won't work; start game isn't sent after login.
		p.Acknowledgement(func() {
			p.Session().GameMode = pk.WorldGameMode
		})
	case *packet.SetPlayerGameType:
		p.Acknowledgement(func() {
			p.Session().GameMode = pk.GameType
		})
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
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
	}
}
