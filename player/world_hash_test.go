package player

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	dfworld "github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestBlockRuntimeIDToNetworkUsesHashMode(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	p.clientUsesBlockNetworkIDHashes = true

	if got := p.BlockRuntimeIDToNetwork(stoneRID); got != stoneHash {
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
	p.GameDat.UseBlockNetworkIDHashes = true

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
	p.clientUsesBlockNetworkIDHashes = true
	p.GameDat.UseBlockNetworkIDHashes = false

	if got := p.BlockRuntimeIDFromClient(stoneHash); got != stoneRID {
		t.Fatalf("client hash translated to %d, want runtime ID %d", got, stoneRID)
	}
	if got := p.BlockRuntimeIDFromClientToBackend(stoneHash); got != stoneRID {
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
	p.clientUsesBlockNetworkIDHashes = true
	p.GameDat.UseBlockNetworkIDHashes = false
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
	p.clientUsesBlockNetworkIDHashes = true
	p.GameDat.UseBlockNetworkIDHashes = false
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
	p.clientUsesBlockNetworkIDHashes = true
	p.GameDat.UseBlockNetworkIDHashes = false
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

func TestRewriteServerInventoryStackUsesRetainedClientMode(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	p.clientUsesBlockNetworkIDHashes = false
	p.GameDat.UseBlockNetworkIDHashes = true
	pk := &packet.InventorySlot{
		NewItem:     protocol.ItemInstance{Stack: protocol.ItemStack{BlockRuntimeID: int32(stoneHash)}},
		StorageItem: protocol.Option(protocol.ItemInstance{Stack: protocol.ItemStack{BlockRuntimeID: int32(stoneHash)}}),
	}

	if !p.rewriteServerItemBlockNetworkIDs(pk) {
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
	if !p.rewriteServerItemBlockNetworkIDs(content) {
		t.Fatal("inventory content was not rewritten")
	}
	if got := uint32(content.Content[0].Stack.BlockRuntimeID); got != stoneRID {
		t.Fatalf("inventory content block ID = %d, want runtime ID %d", got, stoneRID)
	}
	if got := uint32(original[0].Stack.BlockRuntimeID); got != stoneHash {
		t.Fatalf("retained backend inventory block ID = %d, want hash %d", got, stoneHash)
	}
}

func TestRewriteServerRecipesPreservesBackendOutputs(t *testing.T) {
	world.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := world.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := New(slog.New(slog.NewTextHandler(io.Discard, nil)), MonitoringState{CurrentTime: time.Now()}, nil)
	p.clientUsesBlockNetworkIDHashes = true
	p.GameDat.UseBlockNetworkIDHashes = false
	backendRecipe := &protocol.ShapedChemistryRecipe{ShapedRecipe: protocol.ShapedRecipe{
		Output: []protocol.ItemStack{{BlockRuntimeID: int32(stoneRID)}},
	}}
	pk := &packet.CraftingData{Recipes: []protocol.Recipe{backendRecipe}}

	if !p.rewriteServerItemBlockNetworkIDs(pk) {
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
