package player

import (
	"bytes"
	"fmt"
	"math"
	"time"

	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const transferDialTimeout = 30 * time.Second

// TryTransfer attempts to transfer the player to a new upstream server without disconnecting them from this proxy.
func (p *Player) TryTransfer(address string, port uint16) error {
	if p.MState.IsReplay {
		return fmt.Errorf("cannot transfer while replaying")
	}
	if p.conn == nil {
		return fmt.Errorf("client connection is nil")
	}

	targetAddress := fmt.Sprintf("%s:%d", address, port)
	clientData := p.ClientDat
	clientData.ThirdPartyName = p.IdentityDat.DisplayName

	dialer := minecraft.Dialer{
		ClientData:   clientData,
		IdentityData: p.IdentityDat,

		DisconnectOnUnknownPackets: false,
		DisconnectOnInvalidPackets: true,
		FlushRate:                  -1,
	}
	serverConn, err := dialer.DialTimeout("raknet", targetAddress, transferDialTimeout)
	if err != nil {
		return err
	}
	if err := serverConn.DoSpawn(); err != nil {
		_ = serverConn.Close()
		return err
	}

	oldServerConn := p.serverConn
	oldDimension := p.GameDat.Dimension

	p.SetServerConn(serverConn)
	p.ACKs().Invalidate()

	p.clearTransferWorld(oldDimension)
	p.clearTransferEntities()
	p.clearTransferBossBars()
	p.clearTransferPlayerList()
	p.clearTransferObjectives()
	p.clearTransferEffects()
	p.clearTransferWeather()

	if w := p.World(); w != nil {
		w.SetSTWTicks(300)
	}
	if wu := p.WorldUpdater(); wu != nil {
		wu.ResetForTransfer()
	}
	if c := p.Combat(); c != nil {
		c.Reset()
	}
	if c := p.ClientCombat(); c != nil {
		c.Reset()
	}

	gameData := p.GameDat
	_ = p.SendPacketToClient(&packet.SetPlayerGameType{
		GameType: p.GameMode,
	})
	_ = p.SendPacketToClient(&packet.GameRulesChanged{
		GameRules: gameData.GameRules,
	})
	_ = p.SendPacketToClient(&packet.SetDifficulty{
		Difficulty: uint32(gameData.Difficulty),
	})
	metadata := protocol.NewEntityMetadata()
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagBreathing)
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagHasGravity)
	_ = p.SendPacketToClient(&packet.SetActorData{
		EntityRuntimeID: p.ClientRuntimeId,
		EntityMetadata:  metadata,
	})

	p.Movement().SetPos(gameData.PlayerPosition.Sub(mgl32.Vec3{0, 1.62, 0}))
	p.Movement().SetVel(mgl32.Vec3{})
	p.Movement().SetOnGround(true)
	p.Movement().SetImmobile(false)
	p.Movement().SetNoClip(false)

	_ = p.SendPacketToClient(&packet.MovePlayer{
		EntityRuntimeID: p.ClientRuntimeId,
		Position:        gameData.PlayerPosition,
		Mode:            packet.MoveModeReset,
		OnGround:        true,
	})

	chunkRadius := int32(p.ChunkRadius())
	if wu := p.WorldUpdater(); wu != nil {
		if wuRadius := wu.ChunkRadius(); wuRadius > 0 {
			chunkRadius = wuRadius
		}
	}
	if chunkRadius <= 0 {
		chunkRadius = 4
	}
	maxChunkRadius := uint8(chunkRadius)
	if chunkRadius > 255 {
		maxChunkRadius = 255
	}

	_ = p.SendPacketToClient(&packet.NetworkChunkPublisherUpdate{
		Position: protocol.BlockPos{
			int32(math.Floor(float64(gameData.PlayerPosition.X()))) >> 4,
			0,
			int32(math.Floor(float64(gameData.PlayerPosition.Z()))) >> 4,
		},
		Radius: uint32(chunkRadius * 16),
	})
	_ = serverConn.WritePacket(&packet.RequestChunkRadius{
		ChunkRadius:    chunkRadius,
		MaxChunkRadius: maxChunkRadius,
	})

	_ = p.Flush()
	_ = serverConn.Flush()

	if oldServerConn != nil && oldServerConn != serverConn {
		_ = oldServerConn.Close()
	}
	return nil
}

func (p *Player) clearTransferWorld(oldDimension int32) {
	positions := p.World().ChunkPositions()
	if len(positions) == 0 {
		return
	}

	dimension, ok := df_world.DimensionByID(int(oldDimension))
	if !ok {
		dimension = df_world.Overworld
	}
	subChunkCount, emptyPayload := makeEmptyChunkPayload(dimension.Range())
	for _, chunkPos := range positions {
		_ = p.SendPacketToClient(&packet.LevelChunk{
			Position:      chunkPos,
			Dimension:     oldDimension,
			SubChunkCount: subChunkCount,
			RawPayload:    emptyPayload,
		})
	}
	p.World().PurgeChunks()
}

func (p *Player) clearTransferEntities() {
	removed := make(map[uint64]struct{}, len(p.EntityTracker().All())+len(p.ClientEntityTracker().All()))
	remove := func(rid uint64) {
		if rid == p.ClientRuntimeId {
			return
		}
		if _, ok := removed[rid]; ok {
			return
		}
		removed[rid] = struct{}{}
		_ = p.SendPacketToClient(&packet.RemoveActor{
			EntityUniqueID: int64(rid),
		})
	}

	for rid := range p.EntityTracker().All() {
		remove(rid)
		p.EntityTracker().RemoveEntity(rid)
	}
	for rid := range p.ClientEntityTracker().All() {
		remove(rid)
		p.ClientEntityTracker().RemoveEntity(rid)
	}
}

func (p *Player) clearTransferBossBars() {
	if len(p.transferBossBars) == 0 {
		return
	}
	for bossID := range p.transferBossBars {
		_ = p.SendPacketToClient(&packet.BossEvent{
			BossEntityUniqueID: bossID,
			EventType:          packet.BossEventHide,
		})
		delete(p.transferBossBars, bossID)
	}
}

func (p *Player) clearTransferPlayerList() {
	if len(p.transferPlayerList) == 0 {
		return
	}

	entries := make([]protocol.PlayerListEntry, 0, len(p.transferPlayerList))
	for uuid := range p.transferPlayerList {
		entries = append(entries, protocol.PlayerListEntry{UUID: uuid})
		delete(p.transferPlayerList, uuid)
	}
	_ = p.SendPacketToClient(&packet.PlayerList{
		ActionType: packet.PlayerListActionRemove,
		Entries:    entries,
	})
}

func (p *Player) clearTransferObjectives() {
	if len(p.transferObjectives) == 0 {
		return
	}
	for objective := range p.transferObjectives {
		_ = p.SendPacketToClient(&packet.RemoveObjective{
			ObjectiveName: objective,
		})
		delete(p.transferObjectives, objective)
	}
}

func (p *Player) clearTransferEffects() {
	for id := range p.Effects().All() {
		_ = p.SendPacketToClient(&packet.MobEffect{
			EntityRuntimeID: p.ClientRuntimeId,
			Operation:       packet.MobEffectRemove,
			EffectType:      id,
		})
		p.Effects().Remove(id)
	}
}

func (p *Player) clearTransferWeather() {
	_ = p.SendPacketToClient(&packet.LevelEvent{
		EventType: packet.LevelEventStopThunderstorm,
		EventData: 0,
	})
	_ = p.SendPacketToClient(&packet.LevelEvent{
		EventType: packet.LevelEventStopRaining,
		EventData: 10000,
	})
}

func makeEmptyChunkPayload(dimRange df_cube.Range) (subChunkCount uint32, payload []byte) {
	c := chunk.New(oworld.AirRuntimeID, dimRange)
	data := chunk.Encode(c, chunk.NetworkEncoding)

	buf := bytes.NewBuffer(nil)
	for _, sub := range data.SubChunks {
		buf.Write(sub)
	}
	buf.Write(data.Biomes)
	buf.WriteByte(0)
	return uint32(len(data.SubChunks)), buf.Bytes()
}

func (p *Player) translateClientPacketForTransfer(pk packet.Packet) (modified bool) {
	if !p.IDModified {
		return false
	}

	switch pk := pk.(type) {
	case *packet.MobEquipment:
		pk.EntityRuntimeID = p.RuntimeId
		return true
	case *packet.InventoryTransaction:
		if dat, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			switch dat.TargetEntityRuntimeID {
			case math.MaxInt64:
				dat.TargetEntityRuntimeID = p.ClientRuntimeId
				return true
			case p.ClientRuntimeId:
				dat.TargetEntityRuntimeID = p.RuntimeId
				return true
			}
		}
	case *packet.Respawn:
		pk.EntityRuntimeID = p.RuntimeId
		return true
	case *packet.Animate:
		pk.EntityRuntimeID = p.RuntimeId
		return true
	case *packet.MovePlayer:
		pk.EntityRuntimeID = p.RuntimeId
		return true
	case *packet.Interact:
		switch pk.TargetEntityRuntimeID {
		case math.MaxInt64:
			pk.TargetEntityRuntimeID = p.ClientRuntimeId
			return true
		case p.ClientRuntimeId:
			pk.TargetEntityRuntimeID = p.RuntimeId
			return true
		}
	case *packet.PlayerAction:
		pk.EntityRuntimeID = p.RuntimeId
		return true
	case *packet.ContainerOpen:
		if pk.ContainerEntityUniqueID == math.MaxInt64 {
			pk.ContainerEntityUniqueID = p.ClientUniqueId
			return true
		}
	}
	return false
}

func (p *Player) translateServerPacketForTransfer(pk packet.Packet) (cancel bool, modified bool) {
	if !p.IDModified {
		return false, false
	}

	switch pk := pk.(type) {
	case *packet.Animate:
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.ActorEvent:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.AddPlayer:
		if pk.EntityRuntimeID == p.RuntimeId {
			return true, false
		}
		if pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
	case *packet.AddActor:
		if pk.EntityRuntimeID == p.RuntimeId {
			return true, false
		}
		if pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
	case *packet.MoveActorAbsolute:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.MovePlayer:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.SetActorData:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.UpdateAttributes:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.MobEffect:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.Respawn:
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.AddItemActor:
		if pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
	case *packet.MobEquipment:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.MobArmourEquipment:
		if pk.EntityRuntimeID != p.RuntimeId && pk.EntityRuntimeID == p.ClientRuntimeId {
			pk.EntityRuntimeID = math.MaxInt64
			return false, true
		}
		if pk.EntityRuntimeID == p.RuntimeId {
			pk.EntityRuntimeID = p.ClientRuntimeId
			return false, true
		}
	case *packet.RemoveActor:
		if pk.EntityUniqueID == p.ClientUniqueId {
			pk.EntityUniqueID = math.MaxInt64
			return false, true
		}
	case *packet.ContainerOpen:
		if pk.ContainerEntityUniqueID == p.ClientUniqueId {
			pk.ContainerEntityUniqueID = math.MaxInt64
			return false, true
		}
	}
	return false, false
}
