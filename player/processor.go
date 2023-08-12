package player

import (
	"bytes"
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

		defer func() {
			p.mInfo.Teleporting = false
			p.SetRespawned(false)

			if p.movementMode == utils.ModeSemiAuthoritative {
				p.setMovementToClient()
			}
		}()

		p.cleanChunks()
		prevPos := p.mInfo.ServerPosition
		p.handlePlayerAuthInput(pk)

		if p.combatMode == utils.ModeFullAuthoritative {
			p.validateCombat(prevPos.Add(mgl32.Vec3{0, p.eyeOffset, 0}))
		} else {
			p.tickEntitiesPos()
		}

		if acks := p.Acknowledgements(); acks != nil {
			acks.HasTicked = true
		}
		p.needsCombatValidation = false
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
		if cmd[0] == "!oomph_debug" {
			if len(cmd) != 3 {
				p.SendOomphDebug("Usage: oomph_debug <mode> <value>", packet.TextTypeChat)
				return true
			}

			b := cmd[2] == "on" || cmd[2] == "true"

			switch cmd[1] {
			case "latency":
				p.debugger.LogLatency = b
			case "combat_data":
				p.debugger.LogCombatData = b
			case "server_knockback":
				p.debugger.UseServerKnockback = b
			case "buffer_info":
				p.debugger.UsePacketBuffer = b
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
				p.debugger.LogMovementPredictions = b
			default:
				p.SendOomphDebug("Unknown debug mode: "+cmd[1], packet.TextTypeChat)
				return true
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
			e := entity.NewEntity(
				pk.Position,
				pk.Velocity,
				mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw},
				true,
			)
			e.SetPositionBufferSize(uint64(p.combatNetworkCutoff))
			p.AddEntity(pk.EntityRuntimeID, e)
		})
	case *packet.AddActor:
		if pk.EntityRuntimeID == p.rid {
			// We are the player.
			return false
		}

		p.Acknowledgement(func() {
			e := entity.NewEntity(
				pk.Position,
				pk.Velocity,
				mgl32.Vec3{pk.Pitch, pk.HeadYaw, pk.Yaw},
				false,
			)
			e.SetPositionBufferSize(uint64(p.combatNetworkCutoff))
			p.AddEntity(pk.EntityRuntimeID, e)
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

		pk.Tick = 0 // prevent any rewind from being done
		p.Acknowledgement(func() {
			p.Teleport(pk.Position)
		})
	case *packet.SetActorData:
		pk.Tick = 0 // prevent any rewind from being done

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
				p.chunks = make(map[protocol.ChunkPos]*chunk.Chunk)
				p.inLoadedChunk = false
				p.inLoadedChunkTicks = 0
				p.chkMu.Unlock()

				break
			}
		}

		p.Acknowledgement(func() {
			for _, a := range pk.Attributes {
				if a.Name == "minecraft:movement" {
					p.miMu.Lock()
					p.mInfo.MovementSpeed = a.Value
					p.miMu.Unlock()

					break
				}
			}
		})
	case *packet.SetActorMotion:
		if pk.EntityRuntimeID != p.rid {
			return false
		}

		// If the player is behind by more than the knockback network cutoff, then instantly set the KB
		// of the player instead of waiting for an acknowledgement. This will ensure that players
		// with very high latency do not get a significant advantage due to them receiving knockback late.
		if (p.movementMode == utils.ModeFullAuthoritative && p.TickLatency() >= p.knockbackNetworkCutoff) || p.debugger.UseServerKnockback {
			p.SetKnockback(pk.Velocity)
			return false
		}

		// Send an acknowledgement to the player to get the client tick where the player will apply KB and verify that the client
		// does take knockback when it recieves it.
		p.Acknowledgement(func() {
			p.SetKnockback(pk.Velocity)
		})
	case *packet.LevelChunk:
		p.handleLevelChunk(pk)
	case *packet.SubChunk:
		p.Acknowledgement(func() {
			p.handleSubChunk(pk)
		})
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
		p.Disconnect(pk.Message)
		return true
	}
	return false
}

// handlePlayerAuthInput processes the input packet sent by the client to the server. This also updates some of the movement states such as
// if the player is sprinting, jumping, or in a loaded chunk.
func (p *Player) handlePlayerAuthInput(pk *packet.PlayerAuthInput) {
	p.miMu.Lock()
	defer p.miMu.Unlock()

	p.nextTickActionsMu.Lock()
	for _, fn := range p.nextTickActions {
		fn()
	}
	p.nextTickActions = make([]func(), 0)
	p.nextTickActionsMu.Unlock()

	// Update the input mode of the player. This is mainly used for combat detections.
	// Note while this can be abused, techincally, there are still combat checks in place for touch players.
	p.inputMode = pk.InputMode

	// Call p.Move() to update some movement states, such as the client predicted movement.
	p.Move(pk)

	// If the movement mode is set to client authoritative, then we do not need to do any processing
	// of movement, other than sending it to the server.
	if p.movementMode == utils.ModeClientAuthoritative {
		p.setMovementToClient()
		return
	}

	// Set the last used input of the player to the current input. This will execute at the end of the function.
	defer func() {
		p.mInfo.LastUsedInput = pk
	}()

	// Determine wether the player's current position has a chunk.
	p.inLoadedChunk = p.ChunkExists(protocol.ChunkPos{
		int32(math32.Floor(p.mInfo.ServerPosition[0])) >> 4,
		int32(math32.Floor(p.mInfo.ServerPosition[2])) >> 4,
	})

	// Check if the player has swung their arm into the air, and if so handle it by registering it as a click.
	if utils.HasFlag(pk.InputData, packet.InputFlagMissedSwing) {
		p.Click()
		p.updateCombatData(nil)
	}

	// The client is doing a block action on it's side, so we want to replicate this to
	// make the copy of the server and client world identical.
	if utils.HasFlag(pk.InputData, packet.InputFlagPerformBlockActions) && p.movementMode != utils.ModeClientAuthoritative {
		for _, action := range pk.BlockActions {
			// If we are in direct mode using dragonfly, server authoritative block breaking is enabled.
			if p.serverConn == nil || p.serverConn.GameData().PlayerMovementSettings.ServerAuthoritativeBlockBreaking {
				if action.Action != protocol.PlayerActionPredictDestroyBlock {
					continue
				}

				p.networkClientBreaksBlock(action.BlockPos)
				continue
			}

			// If server authoritaitve block breaking is disabled, the behavior in PlayerAuthInput for breaking blocks is different.
			switch action.Action {
			case protocol.PlayerActionStartBreak:
				if p.breakingBlockPos != nil {
					continue
				}

				p.breakingBlockPos = &action.BlockPos
			case protocol.PlayerActionCrackBreak:
				if p.breakingBlockPos == nil {
					continue
				}

				if *p.breakingBlockPos == action.BlockPos {
					continue
				}

				p.Disconnect(game.ErrorInvalidBlockBreak)
			case protocol.PlayerActionAbortBreak:
				p.breakingBlockPos = nil
			case protocol.PlayerActionStopBreak:
				p.networkClientBreaksBlock(*p.breakingBlockPos)
				p.breakingBlockPos = nil
			}
		}
	}

	// UpdateMovementStates updates the movement states of the player.
	p.updateMovementStates(pk)
	// Run the movement simulation of the player.
	p.doMovementSimulation()

	// If the movement mode is set to be full server authoritative, then we want to set the position of the player
	// to the server predicted position.
	if p.movementMode == utils.ModeFullAuthoritative {
		pk.Position = p.mInfo.ServerPosition.Add(mgl32.Vec3{0, 1.62})
	}
}

// handleLevelChunk handles all LevelChunk packets sent by the server. This is used to create a copy
// of the client world on our end.
func (p *Player) handleLevelChunk(pk *packet.LevelChunk) {
	// Check if this LevelChunk packet is compatiable with oomph's handling.
	if pk.SubChunkCount == protocol.SubChunkRequestModeLimited || pk.SubChunkCount == protocol.SubChunkRequestModeLimitless {
		return
	}

	// If the movement mode is client authoritative, we will not be needing a copy of the client's world.
	if p.movementMode == utils.ModeClientAuthoritative {
		return
	}

	// Decode the chunk data, and remove any uneccessary data via. Compact().
	c, err := chunk.NetworkDecode(air, pk.RawPayload, int(pk.SubChunkCount), world.Overworld.Range())
	if err != nil {
		c = chunk.New(air, world.Overworld.Range())
	}
	c.Compact()

	p.Acknowledgement(func() {
		p.AddChunk(c, pk.Position)
	})
}

// handleSubChunk handles all SubChunk packets sent by the server. This is used to create a copy
// of the client world on our end.
func (p *Player) handleSubChunk(pk *packet.SubChunk) {
	for _, entry := range pk.SubChunkEntries {
		// Do not handle sub-chunk responses that returned an error.
		if entry.Result != protocol.SubChunkResultSuccess {
			continue
		}

		chunkPos := protocol.ChunkPos{
			pk.Position[0] + int32(entry.Offset[0]),
			pk.Position[2] + int32(entry.Offset[2]),
		}

		var c *chunk.Chunk
		c, ok := p.Chunk(chunkPos)

		// If the chunk doesn't already exist in the player map, it is being sent to the client
		// for the first time, so we create a new one.
		if !ok {
			c = chunk.New(air, dimensionFromNetworkID(pk.Dimension).Range())
		}

		var index byte
		sub, err := chunk_subChunkDecode(bytes.NewBuffer(entry.RawPayload), c, &index, chunk.NetworkEncoding)
		if err != nil {
			panic(err)
		}

		c.Sub()[index] = sub

		p.AddChunk(c, chunkPos)
	}
}

// handleBlockPlace handles when a player sends a block placement to the server. This function
// will place a block on the specified position to account for the temporary client-server desync
// between worlds when blocks are placed.
func (p *Player) handleBlockPlace(t *protocol.UseItemTransactionData) bool {
	i, ok := world.ItemByRuntimeID(t.HeldItem.Stack.NetworkID, int16(t.HeldItem.Stack.MetadataValue))
	if !ok {
		return false
	}

	// Determine if the item can be placed as a block.
	b, ok := i.(world.Block)
	if !ok {
		return false
	}

	// Find the replace position of the block. This will be used if the block at the current position
	// is replacable (e.g: water, lava, air).
	replacePos := utils.BlockToCubePos(t.BlockPosition)
	fb := p.Block(replacePos)

	// If the block at the position is not replacable, we want to place the block on the side of the block.
	if replaceable, ok := fb.(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
		replacePos = replacePos.Side(cube.Face(t.BlockFace))
	}

	// Make a list of BBoxes the block will occupy.
	bx := b.Model().BBox(df_cube.Pos(replacePos), nil)
	boxes := make([]cube.BBox, 0)
	for _, bxx := range bx {
		boxes = append(boxes, game.DFBoxToCubeBox(bxx).Translate(mgl32.Vec3{
			float32(replacePos.X()),
			float32(replacePos.Y()),
			float32(replacePos.Z()),
		}))
	}

	// Get the player's AABB and translate it to the position of the player. Then check if it intersects
	// with any of the boxes the block will occupy. If it does, we don't want to place the block.
	if cube.AnyIntersections(boxes, p.AABB()) {
		return false
	}

	for _, e := range p.entities {
		ebb := e.AABB().Translate(e.Position())
		if cube.AnyIntersections(boxes, ebb) {
			return false
		}
	}

	// Set the block in the world
	p.SetBlock(replacePos, b)
	return false
}

// handleMobEffect handles the MobEffect packet sent by the server. This is used to apply effects
// to the player.
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

// handleSetActorData handles the SetActorData packet sent by the server. This is used to update
// some of the player's metadata.
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
		if isPlayer {
			e.SetAABB(game.AABBFromDimensions(width.(float32), e.AABB().Height()))
		}
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
	p.mInfo.Sprinting = utils.HasDataFlag(entity.DataFlagSprinting, flags)
	p.mInfo.Sneaking = utils.HasDataFlag(entity.DataFlagSneaking, flags)
	p.mInfo.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, flags)
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
