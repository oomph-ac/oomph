package player

import (
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/utils"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// WorldUpdaterComponent is a component that handles block and chunk updates to the world of the member player.
type WorldUpdaterComponent interface {
	// HandleLevelChunk allows the world updater component to handle a LevelChunk packet sent by the server.
	HandleLevelChunk(pk *packet.LevelChunk)
	// HandleSubChunk allows the world updater component to handle a SubChunk packet sent by the server.
	HandleSubChunk(pk *packet.SubChunk)
	// AttemptBlockPlacement attempts a block placement request from the client. It returns false if the simulation is unable
	// to place a block at the given position.
	AttemptBlockPlacement(pk *packet.InventoryTransaction) bool
	// ValidateInteraction validates if a player is allowed to perform an action on an interactable block.
	ValidateInteraction(pk *packet.InventoryTransaction) bool

	// SetChunkRadius sets the chunk radius of the world updater component.
	SetChunkRadius(radius int32)
	// ChunkRadius returns the chunk radius of the world udpater component.
	ChunkRadius() int32

	// DeferChunk defers a chunk that isn't in range of the WorldLoader.
	DeferChunk(pos protocol.ChunkPos, c *chunk.Chunk)
	// ChunkDeferred returns true and the chunk if the chunk is deferred.
	ChunkDeferred(pos protocol.ChunkPos) (*chunk.Chunk, bool)

	// ChunkPending returns true if a chunk position is pending.
	ChunkPending(pos protocol.ChunkPos) bool
	// Generate is a method used by Dragonfly to generate a chunk at a specific position for it's world. We use the world updater
	// component to track this to know when a chunk should be set as pending.
	GenerateChunk(pos world.ChunkPos, chunk *chunk.Chunk)

	// SetBlockBreakPos sets the block breaking pos of the world updater component.
	SetBlockBreakPos(pos *protocol.BlockPos)
	// BlockBreakPos returns the block breaking pos of the world updater component.
	BlockBreakPos() *protocol.BlockPos

	// Tick ticks the world updater component.
	Tick()
}

func (p *Player) SetWorldUpdater(c WorldUpdaterComponent) {
	p.worldUpdater = c
}

func (p *Player) WorldUpdater() WorldUpdaterComponent {
	return p.worldUpdater
}

func (p *Player) World() *world.World {
	return p.world
}

func (p *Player) WorldLoader() *world.Loader {
	return p.worldLoader
}

func (p *Player) WorldTx() *world.Tx {
	return p.worldTx
}

func (p *Player) RegenerateWorld() {
	if p.worldTx != nil {
		panic(oerror.New("cannot regenerate world while transaction is in effect"))
	}
	newWorld := world.Config{
		ReadOnly:        true,
		Generator:       p.worldUpdater,
		SaveInterval:    -1,
		RandomTickSpeed: -1,
		Dim:             oworld.Overworld,
	}.New()
	newWorld.StopWeatherCycle()
	newWorld.StopTime()
	if w := p.world; w != nil {
		if p.worldLoader == nil {
			panic(oerror.New("world loader should not be null when world is not null"))
		}
		<-w.Exec(func(tx *world.Tx) {
			p.worldLoader.ChangeWorld(tx, newWorld)
		})
		w.Close()
		return
	}

	if p.worldLoader != nil {
		panic(oerror.New("world loader should be null when world is null"))
	}
	p.world = newWorld
	p.worldLoader = world.NewLoader(16, p.world, p)
	p.log.Info("created world loader")
}

func (p *Player) SyncWorld() {
	// Update the blocks in the world so the client can sync itself properly.
	for _, blockResult := range utils.GetNearbyBlocks(p.Movement().BoundingBox(), true, true, p.worldTx) {
		p.SendPacketToClient(&packet.UpdateBlock{
			Position: protocol.BlockPos{
				int32(blockResult.Position[0]),
				int32(blockResult.Position[1]),
				int32(blockResult.Position[2]),
			},
			NewBlockRuntimeID: world.BlockRuntimeID(blockResult.Block),
			Flags:             packet.BlockUpdateNeighbours,
			Layer:             0, // TODO: Implement and account for multi-layer blocks.
		})
	}
}

func (p *Player) handleBlockActions(pk *packet.PlayerAuthInput) {
	/* chunkPos := protocol.ChunkPos{
		int32(math32.Floor(p.movement.Pos().X())) >> 4,
		int32(math32.Floor(p.movement.Pos().Z())) >> 4,
	} */

	if pk.InputData.Load(packet.InputFlagPerformBlockActions) {
		for _, action := range pk.BlockActions {
			switch action.Action {
			case protocol.PlayerActionPredictDestroyBlock:
				if p.ServerConn() == nil {
					continue
				}

				if !p.ServerConn().GameData().PlayerMovementSettings.ServerAuthoritativeBlockBreaking {
					continue
				}

				p.worldTx.SetBlock(df_cube.Pos{
					int(action.BlockPos.X()),
					int(action.BlockPos.Y()),
					int(action.BlockPos.Z()),
				}, block.Air{}, nil)
			case protocol.PlayerActionStartBreak:
				if p.worldUpdater.BlockBreakPos() != nil {
					continue
				}

				p.worldUpdater.SetBlockBreakPos(&action.BlockPos)
			case protocol.PlayerActionCrackBreak:
				if p.worldUpdater.BlockBreakPos() == nil {
					continue
				}

				p.worldUpdater.SetBlockBreakPos(&action.BlockPos)
			case protocol.PlayerActionAbortBreak:
				p.worldUpdater.SetBlockBreakPos(nil)
			case protocol.PlayerActionStopBreak:
				if p.worldUpdater.BlockBreakPos() == nil {
					continue
				}

				p.worldTx.SetBlock(df_cube.Pos{
					int(p.worldUpdater.BlockBreakPos().X()),
					int(p.worldUpdater.BlockBreakPos().Y()),
					int(p.worldUpdater.BlockBreakPos().Z()),
				}, block.Air{}, nil)
			}
		}
	}

	/* if p.Ready {
		p.World().CleanChunks(p.worldUpdater.ChunkRadius(), chunkPos)
	} */
}
