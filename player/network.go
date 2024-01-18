package player

import (
	"context"
	"net"
	"time"

	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func VersionInRange(version int, min int, max int) bool {
	return version >= min && version <= max
}

// Conn returns the connection to the client.
func (p *Player) Conn() *minecraft.Conn {
	return p.conn
}

// ServerConn returns the connection to the server.
func (p *Player) ServerConn() *minecraft.Conn {
	return p.serverConn
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

// WritePacket will call minecraft.Conn.WritePacket and process the packet with oomph.
func (p *Player) WritePacket(pk packet.Packet) error {
	if p.conn == nil {
		return oerror.New("conn is nil in session")
	}

	p.HandleFromServer(pk)
	return nil
}

// ReadPacket will call minecraft.Conn.ReadPacket and process the packet with oomph.
func (p *Player) ReadPacket() (pk packet.Packet, err error) {
	if p.conn == nil {
		return pk, oerror.New("conn is nil in session")
	}

	// Oomph wants to send a packet to the server here.
	if len(p.packetQueue) > 0 {
		pk = p.packetQueue[0]
		p.packetQueue = p.packetQueue[1:]
		return pk, nil
	}

	pk, err = p.conn.ReadPacket()
	if err != nil {
		return pk, err
	}
	p.HandleFromClient(pk)

	// Check if the packet queue is empty. If it is not, return the first packet in the queue.
	if len(p.packetQueue) == 0 {
		return p.ReadPacket()
	}

	pk = p.packetQueue[0]
	p.packetQueue = p.packetQueue[1:]
	return pk, nil
}

// StartGameContext starts the game for the conn with a context to cancel it.
func (p *Player) StartGameContext(ctx context.Context, data minecraft.GameData) error {
	data.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	data.PlayerMovementSettings.RewindHistorySize = 100

	return p.conn.StartGameContext(ctx, data)
}
