package player

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// SendPacketToServer sends a packet to the server
func (p *Player) SendPacketToServer(pk packet.Packet) error {
	if p.serverConn == nil {
		p.tMu.Lock()
		p.toSend = append(p.toSend, pk)
		p.tMu.Unlock()

		return nil
	}

	return p.ServerConn().WritePacket(pk)
}
