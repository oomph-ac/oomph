package context

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// HandlePacketContext is a context that is passed to Oomph's HandleClientPacket and HandleServerPacket functions.
type HandlePacketContext struct {
	// pk is a pointer to the underlying packet that is being handled.
	pk *packet.Packet
	// modified is a boolean indicating whether Oomph has modified the packet. If it has, we have to re-encode
	// the packet with the modified data.
	modified bool
	// cancel is a boolean indicating wether the packet should be sent to the client/server.
	cancel bool
}

func NewHandlePacketContext(pk *packet.Packet) *HandlePacketContext {
	return &HandlePacketContext{pk: pk}
}

func (ctx *HandlePacketContext) Packet() *packet.Packet {
	return ctx.pk
}

func (ctx *HandlePacketContext) Modified() bool {
	return ctx.modified
}

func (ctx *HandlePacketContext) SetModified() {
	ctx.modified = true
}

func (ctx *HandlePacketContext) Cancelled() bool {
	return ctx.cancel
}

func (ctx *HandlePacketContext) Cancel() {
	ctx.cancel = true
}
