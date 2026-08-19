package player

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/anticheat/world/blocknetwork"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Conn returns the connection to the client.
func (p *Player) Conn() *minecraft.Conn {
	return p.conn
}

// ServerConn returns the connection to the server.
func (p *Player) ServerConn() ServerConn {
	return p.serverConn
}

// SetConn sets the connection to the client.
func (p *Player) SetConn(conn *minecraft.Conn) {
	p.conn = conn

	p.RuntimeId = conn.GameData().EntityRuntimeID
	p.UniqueId = conn.GameData().EntityUniqueID

	p.ClientDat = conn.ClientData()
	p.IdentityDat = conn.IdentityData()
	p.GameDat = conn.GameData()
	p.Version = conn.Proto().ID()
	p.dimension = p.GameDat.Dimension
}

// SetServerConn sets the connection to the server.
func (p *Player) SetServerConn(conn ServerConn) {
	if conn == nil {
		p.Disconnect("<red>Proxy was unable to complete transfer to remote server.</red>")
		return
	}

	blockNetwork := blocknetwork.NewCodec(p.World().BlockRegistry(), blocknetwork.ModeFromHashes(conn.GameData().UseBlockNetworkIDHashes))
	if p.serverConn == nil {
		p.blockNetwork = blockNetwork
		for _, item := range conn.GameData().Items {
			if i, ok := world.ItemByName(item.Name, 0); ok {
				p.items[item.RuntimeID] = i
			}
		}
	}

	p.GameDat = conn.GameData()
	p.setDimension(p.GameDat.Dimension)
	p.serverConn = conn
	p.RuntimeId = conn.GameData().EntityRuntimeID
	p.UniqueId = conn.GameData().EntityUniqueID
	p.GameMode = conn.GameData().PlayerGameMode
	if p.GameMode == 5 {
		p.GameMode = conn.GameData().WorldGameMode
	}

	p.PendingCorrectionACK = false
	p.acks.ResetTransferState()
	p.movement.ResetTransferState(p.GameDat.PlayerPosition)
}

// BlockNetwork returns the codec shared by the client and every backend in this session.
func (p *Player) BlockNetwork() blocknetwork.Codec {
	return p.blockNetwork
}

// BackendTransferState contains client-visible state that must be cleared when
// a proxy switches this player to another backend.
type BackendTransferState struct {
	EffectIDs []int32
}

// TransferServerConn atomically installs a backend and clears state owned by
// the previous backend. It uses the same processing lock as packet handling and
// Tick, so no component can observe a partially reset transfer.
func (p *Player) TransferServerConn(conn ServerConn) (BackendTransferState, error) {
	p.procMu.Lock()
	defer p.procMu.Unlock()

	state := BackendTransferState{EffectIDs: make([]int32, 0, len(p.effects.All()))}
	targetMode := blocknetwork.ModeFromHashes(conn.GameData().UseBlockNetworkIDHashes)
	if targetMode != p.blockNetwork.Mode() {
		return state, fmt.Errorf("backend block-hash setting %t does not match session setting %t", targetMode == blocknetwork.Hashes, p.blockNetwork.Mode() == blocknetwork.Hashes)
	}
	for effectID := range p.effects.All() {
		state.EffectIDs = append(state.EffectIDs, effectID)
	}
	p.SetServerConn(conn)
	p.world.PurgeChunks()
	for rid := range p.entTracker.All() {
		p.entTracker.RemoveEntity(rid)
	}
	for rid := range p.clientEntTracker.All() {
		p.clientEntTracker.RemoveEntity(rid)
	}
	p.effects.RemoveAll()
	p.combat.Reset()
	p.clientCombat.Reset()
	return state, nil
}

// ChunkRadius returns the chunk radius as requested by the client at the other end of the conn.
func (p *Player) ChunkRadius() int {
	return p.conn.ChunkRadius()
}

// ClientCacheEnabled specifies if the conn has the client cache, used for caching chunks client-side, enabled or
// not. Some platforms, like the Nintendo Switch, have this disabled at all times.
func (p *Player) ClientCacheEnabled() bool {
	// todo: support client cache
	//return p.conn.ClientCacheEnabled()
	return false
}

// ReadPacket reads a packet from the connection.
func (p *Player) ReadPacket() (packet.Packet, error) {
	return p.conn.ReadPacket()
}

// WritePacket writes a packet to the connection.
func (p *Player) WritePacket(pk packet.Packet) error {
	return p.conn.WritePacket(pk)
}

// IdentityData returns the login.IdentityData of a player. It contains the UUID, XUID and username of the connection.
func (p *Player) IdentityData() login.IdentityData {
	return p.conn.IdentityData()
}

// ClientData returns the login.ClientData of a player. This includes less sensitive data of the player like its skin,
// language code and other non-essential information.
func (p *Player) ClientData() login.ClientData {
	return p.conn.ClientData()
}

// Flush flushes the packets buffered by the conn, sending all of them out immediately.
func (p *Player) Flush() error {
	if p.conn == nil {
		return nil
	}
	return p.conn.Flush()
}

// RemoteAddr returns the remote network address.
func (p *Player) RemoteAddr() net.Addr {
	return p.conn.RemoteAddr()
}

// Latency returns the current latency measured over the conn.
func (p *Player) Latency() time.Duration {
	return p.conn.Latency()
}

// StartGameContext starts the game for the conn with a context to cancel it.
func (p *Player) StartGameContext(ctx context.Context, data minecraft.GameData) error {
	//data.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	data.PlayerMovementSettings.RewindHistorySize = 100
	return p.conn.StartGameContext(ctx, data)
}
