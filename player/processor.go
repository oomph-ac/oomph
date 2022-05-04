package player

import (
  "math"
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

// ClientProcess processes the given packet from the client.
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
		p.clientTick++
		if p.ready {
			p.Move(pk)

			if utils.HasFlag(pk.InputData, packet.InputFlagStartSprinting) || utils.HasFlag(pk.InputData, packet.InputFlagStopSprinting) {
				p.sprinting = !p.sprinting
			} else if utils.HasFlag(pk.InputData, packet.InputFlagStartSneaking) || utils.HasFlag(pk.InputData, packet.InputFlagStopSneaking) {
				p.sneaking = !p.sneaking
			}
			p.jumping = utils.HasFlag(pk.InputData, packet.InputFlagStartJumping)

			p.tickEntityLocations()
			p.teleporting = false
		}
	case *packet.LevelSoundEvent:
		if pk.SoundType == packet.SoundEventAttackNoDamage {
			p.Click()
		}
	case *packet.InventoryTransaction:
		if _, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			p.Click()
		}
	case *packet.AdventureSettings:
		p.flying = utils.HasFlag(uint64(pk.Flags), packet.AdventureFlagFlying)
	case *packet.Respawn:
		if pk.EntityRuntimeID == p.rid && pk.State == packet.RespawnStateClientReadyToSpawn {
			p.dead = false
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
				teleport := utils.HasFlag(uint64(pk.Flags), packet.MoveFlagTeleport)
				p.Teleport(pk.Position, teleport)
				if teleport {
					p.teleporting = true
				}
			})
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position))
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.rid {
			p.Acknowledgement(func() {
				teleport := pk.Mode == packet.MoveModeTeleport
				p.Teleport(pk.Position, teleport)
				if teleport {
					p.teleporting = true
				}
			})
			return false
		}

		p.MoveEntity(pk.EntityRuntimeID, game.Vec32To64(pk.Position))
	case *packet.SetActorData:
		p.Acknowledgement(func() {
			width, widthExists := pk.EntityMetadata[entity.DataKeyBoundingBoxWidth]
			height, heightExists := pk.EntityMetadata[entity.DataKeyBoundingBoxHeight]
			if e, ok := p.SearchEntity(pk.EntityRuntimeID); ok && widthExists && heightExists {
				e.SetAABB(game.AABBFromDimensions(float64(width.(float32)), float64(height.(float32))))
			}

			if f, ok := pk.EntityMetadata[entity.DataKeyFlags]; pk.EntityRuntimeID == p.rid && ok {
				p.immobile = utils.HasDataFlag(entity.DataFlagImmobile, f.(int64))
			}
		})
	case *packet.SubChunk, *packet.LevelChunk:
		p.Acknowledgement(func() {
			p.ready = true
		})
	case *packet.SetPlayerGameType:
		p.Acknowledgement(func() {
			p.gameMode = pk.GameType
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
						p.dead = true
					}
				}
			})
		}
	}
	return false
}
