package player

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	dfworld "github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/world"
	"github.com/oomph-ac/oomph/world/blocknetwork"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func setBlockNetworkModes(p *Player, client, backend blocknetwork.Mode) {
	p.clientBlockNetwork = blocknetwork.NewCodec(world.BlockRegistry, client)
	p.backendBlockNetwork = blocknetwork.NewCodec(world.BlockRegistry, backend)
}

func TestBlockRuntimeIDToClientUsesHashMode(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	p.clientBlockNetwork = blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.Hashes)

	if got := p.BlockRuntimeIDToClient(stoneRID); got != stoneHash {
		t.Fatalf("network block ID = %d, want hash %d", got, stoneHash)
	}
}

func TestReportedBlockNetworkHashesResolve(t *testing.T) {
	world.FinalizeBlockRegistry()
	tests := []struct {
		hash uint32
		want any
	}{
		{hash: 1741778478, want: block.Cobblestone{}},
		{hash: 2150698529, want: block.Stone{}},
	}
	for _, test := range tests {
		runtimeID, ok := world.BlockRegistry.HashToRuntimeID(test.hash)
		if !ok {
			t.Fatalf("reported hash %d did not resolve", test.hash)
		}
		got, ok := world.BlockRegistry.BlockByRuntimeID(runtimeID)
		if !ok {
			t.Fatalf("runtime ID %d for hash %d did not resolve", runtimeID, test.hash)
		}
		if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", test.want) {
			t.Fatalf("hash %d resolved to %T, want %T", test.hash, got, test.want)
		}
	}
}

func TestConvertToStackAcceptsSignedBlockNetworkHash(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok || int32(stoneHash) >= 0 {
		t.Fatalf("stone network hash = %d, want a signed-negative hash", stoneHash)
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	p.backendBlockNetwork = blocknetwork.NewCodec(world.BlockRegistry, blocknetwork.Hashes)

	stack := p.ConvertToStack(protocol.ItemStack{BlockRuntimeID: int32(stoneHash), Count: 1})
	if _, ok := stack.Item().(block.Stone); !ok {
		t.Fatalf("converted item = %T, want block.Stone", stack.Item())
	}
}

func TestBlockRuntimeIDTranslationSeparatesClientAndBackendModes(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)

	if got := p.BlockRuntimeIDFromClient(stoneHash); got != stoneRID {
		t.Fatalf("client hash translated to %d, want runtime ID %d", got, stoneRID)
	}
	if got := p.ClientToBackendBlockNetwork().Translate(stoneHash); got != stoneRID {
		t.Fatalf("client hash translated for backend to %d, want runtime ID %d", got, stoneRID)
	}
}

func TestRewriteClientBlockNetworkIDsCoversAuthInputInteraction(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)
	inputData := protocol.NewBitset(200)
	inputData.Set(packet.InputFlagPerformItemInteraction)
	pk := &packet.PlayerAuthInput{
		InputData: inputData,
		ItemInteractionData: protocol.UseItemTransactionData{
			BlockRuntimeID: stoneHash,
			HeldItem:       protocol.ItemInstance{Stack: protocol.ItemStack{BlockRuntimeID: int32(stoneHash)}},
		},
	}

	if !p.rewriteClientBlockNetworkIDs(pk) {
		t.Fatal("auth input interaction was not rewritten")
	}
	if got := pk.ItemInteractionData.BlockRuntimeID; got != stoneRID {
		t.Fatalf("interaction block ID = %d, want runtime ID %d", got, stoneRID)
	}
	if got := uint32(pk.ItemInteractionData.HeldItem.Stack.BlockRuntimeID); got != stoneRID {
		t.Fatalf("held item block ID = %d, want runtime ID %d", got, stoneRID)
	}
}

func TestRewriteClientBlockNetworkIDsCoversAuthInputCraftResults(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)
	inputData := protocol.NewBitset(200)
	inputData.Set(packet.InputFlagPerformItemStackRequest)
	action := &protocol.CraftResultsDeprecatedStackRequestAction{
		ResultItems: []protocol.ItemStack{{BlockRuntimeID: int32(stoneHash)}},
	}
	pk := &packet.PlayerAuthInput{
		InputData: inputData,
		ItemStackRequest: protocol.ItemStackRequest{
			Actions: []protocol.StackRequestAction{action},
		},
	}

	if !p.rewriteClientBlockNetworkIDs(pk) {
		t.Fatal("auth input craft results were not rewritten")
	}
	if got := uint32(action.ResultItems[0].BlockRuntimeID); got != stoneRID {
		t.Fatalf("craft result block ID = %d, want runtime ID %d", got, stoneRID)
	}
}

func TestRewriteClientBlockNetworkIDsCoversStandaloneCraftResults(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)
	action := &protocol.CraftResultsDeprecatedStackRequestAction{
		ResultItems: []protocol.ItemStack{{BlockRuntimeID: int32(stoneHash)}},
	}
	pk := &packet.ItemStackRequest{Requests: []protocol.ItemStackRequest{{
		Actions: []protocol.StackRequestAction{action},
	}}}

	if !p.rewriteClientBlockNetworkIDs(pk) {
		t.Fatal("standalone craft results were not rewritten")
	}
	if got := uint32(action.ResultItems[0].BlockRuntimeID); got != stoneRID {
		t.Fatalf("craft result block ID = %d, want runtime ID %d", got, stoneRID)
	}
}

func TestRewriteClientBlockNetworkIDsCoversBlockSound(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)
	pk := &packet.LevelSoundEvent{SoundType: packet.SoundEventPlace, ExtraData: int32(stoneHash)}

	if !p.rewriteClientBlockNetworkIDs(pk) {
		t.Fatal("block sound was not rewritten")
	}
	if got := uint32(pk.ExtraData); got != stoneRID {
		t.Fatalf("block sound ID = %d, want runtime ID %d", got, stoneRID)
	}
}

func TestRewriteServerInventoryStackUsesRetainedClientMode(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.RuntimeIDs, blocknetwork.Hashes)
	pk := &packet.InventorySlot{
		NewItem:     protocol.ItemInstance{Stack: protocol.ItemStack{BlockRuntimeID: int32(stoneHash)}},
		StorageItem: protocol.Option(protocol.ItemInstance{Stack: protocol.ItemStack{BlockRuntimeID: int32(stoneHash)}}),
	}

	if !p.rewriteServerBlockNetworkIDs(pk) {
		t.Fatal("inventory slot was not rewritten")
	}
	if got := uint32(pk.NewItem.Stack.BlockRuntimeID); got != stoneRID {
		t.Fatalf("inventory block ID = %d, want runtime ID %d", got, stoneRID)
	}
	storageItem, ok := pk.StorageItem.Value()
	if !ok {
		t.Fatal("inventory storage item was removed")
	}
	if got := uint32(storageItem.Stack.BlockRuntimeID); got != stoneRID {
		t.Fatalf("inventory storage block ID = %d, want runtime ID %d", got, stoneRID)
	}
	original := []protocol.ItemInstance{{Stack: protocol.ItemStack{BlockRuntimeID: int32(stoneHash)}}}
	content := &packet.InventoryContent{Content: original}
	if !p.rewriteServerBlockNetworkIDs(content) {
		t.Fatal("inventory content was not rewritten")
	}
	if got := uint32(content.Content[0].Stack.BlockRuntimeID); got != stoneRID {
		t.Fatalf("inventory content block ID = %d, want runtime ID %d", got, stoneRID)
	}
	if got := uint32(original[0].Stack.BlockRuntimeID); got != stoneHash {
		t.Fatalf("retained backend inventory block ID = %d, want hash %d", got, stoneHash)
	}
}

func TestRewriteBlockNetworkIDsReportsOnlyActualChanges(t *testing.T) {
	world.FinalizeBlockRegistry()
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)

	clientPackets := map[string]packet.Packet{
		"empty inventory transaction": &packet.InventoryTransaction{},
		"empty equipment":             &packet.MobEquipment{},
		"unrelated sound":             &packet.LevelSoundEvent{SoundType: packet.SoundEventAttackNoDamage},
	}
	for name, pk := range clientPackets {
		t.Run("client/"+name, func(t *testing.T) {
			if p.rewriteClientBlockNetworkIDs(pk) {
				t.Fatal("packet reported a block network ID rewrite without changing an ID")
			}
		})
	}

	setBlockNetworkModes(p, blocknetwork.RuntimeIDs, blocknetwork.Hashes)
	serverPackets := map[string]packet.Packet{
		"empty inventory slot":    &packet.InventorySlot{},
		"empty inventory content": &packet.InventoryContent{},
		"empty equipment":         &packet.MobEquipment{},
		"empty player spawn":      &packet.AddPlayer{},
		"empty item actor":        &packet.AddItemActor{},
		"empty creative content":  &packet.CreativeContent{},
		"empty transaction":       &packet.InventoryTransaction{},
		"empty crafting data":     &packet.CraftingData{},
		"unrelated sound":         &packet.LevelSoundEvent{SoundType: packet.SoundEventAttackNoDamage},
	}
	for name, pk := range serverPackets {
		t.Run("server/"+name, func(t *testing.T) {
			if p.rewriteServerBlockNetworkIDs(pk) {
				t.Fatal("packet reported a block network ID rewrite without changing an ID")
			}
		})
	}
}

func TestRewriteServerBlockNetworkIDsCoversBlockEvents(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.RuntimeIDs, blocknetwork.Hashes)

	event := &packet.LevelEvent{EventType: packet.LevelEventParticlesDestroyBlock, EventData: int32(stoneHash)}
	if !p.rewriteServerBlockNetworkIDs(event) {
		t.Fatal("block event was not rewritten")
	}
	if got := uint32(event.EventData); got != stoneRID {
		t.Fatalf("block event ID = %d, want runtime ID %d", got, stoneRID)
	}

	sound := &packet.LevelSoundEvent{SoundType: packet.SoundEventHit, ExtraData: int32(stoneHash)}
	if !p.rewriteServerBlockNetworkIDs(sound) {
		t.Fatal("block sound was not rewritten")
	}
	if got := uint32(sound.ExtraData); got != stoneRID {
		t.Fatalf("block sound ID = %d, want runtime ID %d", got, stoneRID)
	}
}

func TestRewriteServerBlockNetworkIDsCoversFallingBlockMetadata(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.RuntimeIDs, blocknetwork.Hashes)
	backendMetadata := map[uint32]any{protocol.EntityDataKeyVariant: int32(stoneHash)}
	pk := &packet.AddActor{EntityType: "minecraft:falling_block", EntityMetadata: backendMetadata}

	if !p.rewriteServerBlockNetworkIDs(pk) {
		t.Fatal("falling-block metadata was not rewritten")
	}
	if got := uint32(pk.EntityMetadata[protocol.EntityDataKeyVariant].(int32)); got != stoneRID {
		t.Fatalf("falling-block metadata ID = %d, want runtime ID %d", got, stoneRID)
	}
	if got := uint32(backendMetadata[protocol.EntityDataKeyVariant].(int32)); got != stoneHash {
		t.Fatalf("retained backend metadata ID = %d, want hash %d", got, stoneHash)
	}

	unrelated := &packet.AddActor{
		EntityType:     "minecraft:tropicalfish",
		EntityMetadata: map[uint32]any{protocol.EntityDataKeyVariant: int32(stoneHash)},
	}
	if p.rewriteServerBlockNetworkIDs(unrelated) {
		t.Fatal("unrelated entity variant was rewritten")
	}
}

func TestRewriteServerBlockNetworkIDsCoversBlockValuedActorMetadata(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.RuntimeIDs, blocknetwork.Hashes)

	spawnMetadata := map[uint32]any{protocol.EntityDataKeyCarryBlockRuntimeID: int32(stoneHash)}
	spawn := &packet.AddActor{EntityType: "minecraft:enderman", EntityMetadata: spawnMetadata}
	if !p.rewriteServerBlockNetworkIDs(spawn) {
		t.Fatal("carried-block spawn metadata was not rewritten")
	}
	if got := uint32(spawn.EntityMetadata[protocol.EntityDataKeyCarryBlockRuntimeID].(int32)); got != stoneRID {
		t.Fatalf("carried-block spawn metadata ID = %d, want runtime ID %d", got, stoneRID)
	}
	if got := uint32(spawnMetadata[protocol.EntityDataKeyCarryBlockRuntimeID].(int32)); got != stoneHash {
		t.Fatalf("retained backend spawn metadata ID = %d, want hash %d", got, stoneHash)
	}

	updateMetadata := map[uint32]any{
		protocol.EntityDataKeyCarryBlockRuntimeID:  int32(stoneHash),
		protocol.EntityDataKeyDisplayTileRuntimeID: int32(stoneHash),
		protocol.EntityDataKeyVariant:              int32(stoneHash),
	}
	p.entTracker = actorTypeTracker{entity: &entity.Entity{Type: fallingBlockEntityType}}
	update := &packet.SetActorData{EntityRuntimeID: 42, EntityMetadata: updateMetadata}
	if !p.rewriteServerBlockNetworkIDs(update) {
		t.Fatal("block-valued actor metadata update was not rewritten")
	}
	for _, key := range []uint32{protocol.EntityDataKeyCarryBlockRuntimeID, protocol.EntityDataKeyDisplayTileRuntimeID, protocol.EntityDataKeyVariant} {
		if got := uint32(update.EntityMetadata[key].(int32)); got != stoneRID {
			t.Fatalf("actor metadata key %d ID = %d, want runtime ID %d", key, got, stoneRID)
		}
		if got := uint32(updateMetadata[key].(int32)); got != stoneHash {
			t.Fatalf("retained backend metadata key %d ID = %d, want hash %d", key, got, stoneHash)
		}
	}
}

func TestRetainedClientEquipmentIsIndependentOfBackendTranslation(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)
	pk := &packet.MobEquipment{NewItem: protocol.ItemInstance{Stack: protocol.ItemStack{BlockRuntimeID: int32(stoneHash)}}}

	p.retainClientEquipment(pk)
	p.rewriteClientBlockNetworkIDs(pk)

	if got := uint32(pk.NewItem.Stack.BlockRuntimeID); got != stoneRID {
		t.Fatalf("backend equipment block ID = %d, want runtime ID %d", got, stoneRID)
	}
	if got := uint32(p.LastEquipmentData.NewItem.Stack.BlockRuntimeID); got != stoneHash {
		t.Fatalf("retained client equipment block ID = %d, want hash %d", got, stoneHash)
	}
	generated := p.ClientItemForBackend(p.LastEquipmentData.NewItem)
	if got := uint32(generated.Stack.BlockRuntimeID); got != stoneRID {
		t.Fatalf("generated backend equipment block ID = %d, want runtime ID %d", got, stoneRID)
	}
}

type actorTypeTracker struct {
	entity *entity.Entity
}

func (actorTypeTracker) AddEntity(uint64, *entity.Entity)                  {}
func (actorTypeTracker) RemoveEntity(uint64)                               {}
func (t actorTypeTracker) FindEntity(uint64) *entity.Entity                { return t.entity }
func (actorTypeTracker) All() map[uint64]*entity.Entity                    { return nil }
func (actorTypeTracker) MoveEntity(uint64, int64, mgl32.Vec3, bool)        {}
func (actorTypeTracker) HandleMovePlayer(*packet.MovePlayer)               {}
func (actorTypeTracker) HandleMoveActorAbsolute(*packet.MoveActorAbsolute) {}
func (actorTypeTracker) HandleSetActorData(*packet.SetActorData)           {}
func (actorTypeTracker) Tick(int64)                                        {}

func TestRewriteServerRecipesPreservesBackendOutputs(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	setBlockNetworkModes(p, blocknetwork.Hashes, blocknetwork.RuntimeIDs)
	backendRecipe := &protocol.ShapedChemistryRecipe{ShapedRecipe: protocol.ShapedRecipe{
		Output: []protocol.ItemStack{{BlockRuntimeID: int32(stoneRID)}},
	}}
	pk := &packet.CraftingData{Recipes: []protocol.Recipe{backendRecipe}}

	if !p.rewriteServerBlockNetworkIDs(pk) {
		t.Fatal("crafting data was not rewritten")
	}
	clientRecipe := pk.Recipes[0].(*protocol.ShapedChemistryRecipe)
	if got := uint32(clientRecipe.Output[0].BlockRuntimeID); got != stoneHash {
		t.Fatalf("client recipe block ID = %d, want hash %d", got, stoneHash)
	}
	if got := uint32(backendRecipe.Output[0].BlockRuntimeID); got != stoneRID {
		t.Fatalf("retained backend recipe block ID = %d, want runtime ID %d", got, stoneRID)
	}
}
