package replay

import (
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func init() {
	eventPool[IDPacketEvent] = func() Event { return &PacketEvent{} }
}

// PacketEvent is called when a packet is recieved by either the server or the client.
type PacketEvent struct {
	FromServer bool
	Packet     packet.Packet
}

func (ev *PacketEvent) ID() uint16 {
	return IDPacketEvent
}

func (ev *PacketEvent) Marshal(io protocol.IO) {
	io.Bool(&ev.FromServer)

	var pkID uint32
	if ev.Packet != nil {
		pkID = ev.Packet.ID()
	}
	io.Uint32(&pkID)

	if ev.Packet == nil {
		f, ok := minecraft.DefaultProtocol.Packets(!ev.FromServer)[pkID]
		if !ok {
			panic(oerror.New("packet with ID %d not found", pkID))
		}
		ev.Packet = f()
	}
	ev.Packet.Marshal(io)
}
