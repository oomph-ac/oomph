package proxy

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// PacketContext carries a packet through Handler processing. A handler may Cancel the context to
// prevent the packet from being forwarded, replace the packet entirely with SetPacket, or mark the
// context with SetModified after mutating the packet in place.
type PacketContext struct {
	pk        packet.Packet
	cancelled bool
	modified  bool
}

// NewPacketContext returns a PacketContext wrapping pk.
func NewPacketContext(pk packet.Packet) *PacketContext {
	return &PacketContext{pk: pk}
}

// SetPacket replaces the packet that is forwarded and marks the context as modified.
func (ctx *PacketContext) SetPacket(pk packet.Packet) {
	ctx.modified = true
	ctx.pk = pk
}

// Packet returns the packet currently held by the context.
func (ctx *PacketContext) Packet() packet.Packet {
	return ctx.pk
}

// Cancel prevents the packet from being forwarded.
func (ctx *PacketContext) Cancel() {
	ctx.cancelled = true
}

// Cancelled reports whether the packet was cancelled by a handler.
func (ctx *PacketContext) Cancelled() bool {
	return ctx.cancelled
}

// SetModified marks the packet as modified in place.
func (ctx *PacketContext) SetModified() {
	ctx.modified = true
}

// Modified reports whether the packet was replaced or modified in place.
func (ctx *PacketContext) Modified() bool {
	return ctx.modified
}
