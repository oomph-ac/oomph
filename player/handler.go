package player

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// Handler is an interface that can be implemented to have the player handle certain packets and events.
type Handler interface {
	// ID returns a string that identifies the handler.
	ID() string

	// HandleClientPacket handles a packet sent by a Minecraft client. It returns true
	// if the packet handled should be sent to the server and false if it should be dropped.
	HandleClientPacket(pk packet.Packet, p *Player) bool
	// HandleServerPacket handles a packet sent by a Minecraft server. It returns true
	// if the packet handled should be sent to the client and false if it should be dropped.
	HandleServerPacket(pk packet.Packet, p *Player) bool
	// OnTick is called every server tick.
	OnTick(p *Player)
}
