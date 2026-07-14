package component

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	dfworld "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/player"
	playercontext "github.com/oomph-ac/oomph/player/context"
	oomphworld "github.com/oomph-ac/oomph/world"
	"github.com/oomph-ac/oomph/world/blocknetwork"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestWorldUpdaterDecodesHashedLevelChunk(t *testing.T) {
	oomphworld.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := oomphworld.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}

	position := cube.Pos{1, 0, 1}
	c := chunk.New(oomphworld.BlockRegistry, dfworld.Overworld.Range())
	c.SetBlock(uint8(position[0]), int16(position[1]), uint8(position[2]), 0, stoneRID)
	data := chunk.Encode(c, chunk.NetworkEncoding)
	stoneRIDBytes, stoneHashBytes := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	if err := protocol.WriteVarint32(stoneRIDBytes, int32(stoneRID)); err != nil {
		t.Fatal(err)
	}
	if err := protocol.WriteVarint32(stoneHashBytes, int32(stoneHash)); err != nil {
		t.Fatal(err)
	}
	raw := bytes.NewBuffer(nil)
	converted := false
	for _, subChunk := range data.SubChunks {
		hashed := bytes.ReplaceAll(subChunk, stoneRIDBytes.Bytes(), stoneHashBytes.Bytes())
		if !bytes.Equal(hashed, subChunk) {
			converted = true
		}
		raw.Write(hashed)
	}
	if !converted {
		t.Fatal("stone runtime ID not found in encoded chunk")
	}
	raw.Write(data.Biomes)
	raw.WriteByte(0)

	p := player.New(slog.New(slog.NewTextHandler(io.Discard, nil)), player.MonitoringState{CurrentTime: time.Now()}, nil)
	Register(p)
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: true}})
	p.WorldUpdater().HandleLevelChunk(&packet.LevelChunk{
		Position:      protocol.ChunkPos{0, 0},
		Dimension:     packet.DimensionOverworld,
		SubChunkCount: uint32(len(data.SubChunks)),
		RawPayload:    raw.Bytes(),
	})

	got := p.World().Block(position)
	if _, ok := got.(block.Stone); !ok {
		gotRID := dfworld.BlockRuntimeID(got)
		t.Fatalf("block = %T (runtime ID %d), want stone (runtime ID %d, network hash %d)", got, gotRID, stoneRID, stoneHash)
	}
}

func TestWorldUpdaterConvertsHashedBlockUpdate(t *testing.T) {
	oomphworld.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := oomphworld.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := player.New(slog.New(slog.NewTextHandler(io.Discard, nil)), player.MonitoringState{CurrentTime: time.Now()}, nil)
	Register(p)
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: true}})
	updater := p.WorldUpdater().(*WorldUpdaterComponent)
	position := cube.Pos{4, 5, 6}

	updater.HandleUpdateBlock(&packet.UpdateBlock{
		Position:          protocol.BlockPos{4, 5, 6},
		NewBlockRuntimeID: stoneHash,
	})

	if got := updater.batchedBlockUpdates.Blocks()[position]; got != stoneRID {
		t.Fatalf("pending block runtime ID = %d, want %d (network hash %d)", got, stoneRID, stoneHash)
	}
}

func TestClientBlockHashModeSurvivesBackendTransfer(t *testing.T) {
	oomphworld.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := oomphworld.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := player.New(slog.New(slog.NewTextHandler(io.Discard, nil)), player.MonitoringState{CurrentTime: time.Now()}, nil)
	Register(p)
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: true}})
	if got := p.ClientBlockNetwork().Mode(); got != blocknetwork.Hashes {
		t.Fatalf("initial client mode = %v, want Hashes", got)
	}
	if got := p.BackendBlockNetwork().Mode(); got != blocknetwork.Hashes {
		t.Fatalf("initial backend mode = %v, want Hashes", got)
	}
	if p.ClientToBackendBlockNetwork().Required() || p.BackendToClientBlockNetwork().Required() {
		t.Fatal("matching initial endpoint modes require translation")
	}
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: false}})
	if got := p.ClientBlockNetwork().Mode(); got != blocknetwork.Hashes {
		t.Fatalf("client mode after transfer = %v, want retained Hashes", got)
	}
	if got := p.BackendBlockNetwork().Mode(); got != blocknetwork.RuntimeIDs {
		t.Fatalf("backend mode after transfer = %v, want RuntimeIDs", got)
	}
	if !p.ClientToBackendBlockNetwork().Required() || !p.BackendToClientBlockNetwork().Required() {
		t.Fatal("different endpoint modes do not require translation")
	}

	if got := p.BlockRuntimeIDToNetwork(stoneRID); got != stoneHash {
		t.Fatalf("client block ID after transfer = %d, want initial hash %d", got, stoneHash)
	}
}

func TestLevelChunkIsTranslatedToRetainedClientHashModeAfterTransfer(t *testing.T) {
	oomphworld.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := oomphworld.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	position := cube.Pos{1, 0, 1}
	c := chunk.New(oomphworld.BlockRegistry, dfworld.Overworld.Range())
	c.SetBlock(uint8(position[0]), int16(position[1]), uint8(position[2]), 0, stoneRID)
	data := chunk.Encode(c, chunk.NetworkEncoding)
	raw := bytes.NewBuffer(nil)
	for _, sub := range data.SubChunks {
		raw.Write(sub)
	}
	raw.Write(data.Biomes)
	raw.WriteByte(0)

	p := player.New(slog.New(slog.NewTextHandler(io.Discard, nil)), player.MonitoringState{CurrentTime: time.Now()}, nil)
	Register(p)
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: true}})
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: false}})
	pk := packet.Packet(&packet.LevelChunk{
		Position:      protocol.ChunkPos{0, 0},
		Dimension:     packet.DimensionOverworld,
		SubChunkCount: uint32(len(data.SubChunks)),
		RawPayload:    raw.Bytes(),
	})
	ctx := playercontext.NewHandlePacketContext(&pk)
	p.HandleServerPacket(ctx)

	forwarded := pk.(*packet.LevelChunk)
	decoded, err := chunk.NetworkDecode(oomphworld.BlockRegistry, forwarded.RawPayload, int(forwarded.SubChunkCount), dfworld.Overworld.Range())
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.Block(uint8(position[0]), int16(position[1]), uint8(position[2]), 0); got != stoneHash {
		t.Fatalf("forwarded block ID = %d, want client hash %d", got, stoneHash)
	}
	if _, ok := p.World().Block(position).(block.Stone); !ok {
		t.Fatalf("internal block = %T, want block.Stone", p.World().Block(position))
	}
}

func TestUpdateBlockSyncedIsTranslatedToRetainedClientMode(t *testing.T) {
	oomphworld.FinalizeBlockRegistry()
	stoneRID := dfworld.BlockRuntimeID(block.Stone{})
	stoneHash, ok := oomphworld.BlockRegistry.RuntimeIDToHash(stoneRID)
	if !ok {
		t.Fatal("stone has no network hash")
	}
	p := player.New(slog.New(slog.NewTextHandler(io.Discard, nil)), player.MonitoringState{CurrentTime: time.Now()}, nil)
	Register(p)
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: false}})
	p.SetServerConn(hashModeServerConn{data: minecraft.GameData{UseBlockNetworkIDHashes: true}})
	pk := packet.Packet(&packet.UpdateBlockSynced{NewBlockRuntimeID: stoneHash})
	ctx := playercontext.NewHandlePacketContext(&pk)

	p.HandleServerPacket(ctx)

	if got := pk.(*packet.UpdateBlockSynced).NewBlockRuntimeID; got != stoneRID {
		t.Fatalf("forwarded block ID = %d, want client runtime ID %d", got, stoneRID)
	}
}

type hashModeServerConn struct{ data minecraft.GameData }

func (hashModeServerConn) WritePacket(packet.Packet) error { return nil }
func (hashModeServerConn) Close() error                    { return nil }
func (c hashModeServerConn) GameData() minecraft.GameData  { return c.data }
