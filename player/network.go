package player

import (
	"context"
	"net"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
	modified := p.conn != nil
	p.conn = conn

	p.RuntimeId = conn.GameData().EntityRuntimeID
	p.ClientRuntimeId = conn.GameData().EntityRuntimeID
	p.UniqueId = conn.GameData().EntityUniqueID
	p.ClientUniqueId = conn.GameData().EntityUniqueID
	p.IDModified = modified

	p.ClientDat = conn.ClientData()
	p.IdentityDat = conn.IdentityData()
	p.GameDat = conn.GameData()
	p.Version = conn.Proto().ID()
}

// SetServerConn sets the connection to the server.
func (p *Player) SetServerConn(conn ServerConn) {
	p.serverConn = conn
	p.GameMode = conn.GameData().PlayerGameMode
	if p.GameMode == 5 {
		p.GameMode = conn.GameData().WorldGameMode
	}

	p.RuntimeId = conn.GameData().EntityRuntimeID
	p.UniqueId = conn.GameData().EntityUniqueID
	if !p.IDModified {
		p.ClientRuntimeId = conn.GameData().EntityRuntimeID
		p.ClientUniqueId = conn.GameData().EntityUniqueID
	}

	p.IDModified = true
	p.GameDat = conn.GameData()
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
	data.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	data.PlayerMovementSettings.RewindHistorySize = 100
	p.GameMode = data.PlayerGameMode

	return p.conn.StartGameContext(ctx, data)
}
