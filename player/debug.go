package player

import (
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type AABBDebugDrawer struct {
	Packet  *packet.ClientBoundDebugRenderer
	Expires time.Time
}

func (p *Player) DebugBB(d time.Duration, pk *packet.ClientBoundDebugRenderer) {
	_, found := p.debugDrawers[pk.Position]
	p.debugDrawers[pk.Position] = AABBDebugDrawer{
		Packet:  pk,
		Expires: time.Now().Add(d),
	}

	if !found {
		p.SendPacketToClient(pk)
	}
}
