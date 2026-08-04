package proxy

import (
	"log/slog"

	"github.com/sandertv/gophertunnel/minecraft"
)

// HandlerFactory creates the optional packet and backend lifecycle handler for a client.
type HandlerFactory func(HandlerContext) Handler

// HandlerContext contains the connection state available while constructing a Handler.
type HandlerContext struct {
	Client   *minecraft.Conn
	Listener *minecraft.Listener
	Log      *slog.Logger
}

// Handler integrates optional processing with the proxy session. Packet methods receive a
// PacketContext per packet: cancelling it prevents that packet from being forwarded, and
// SetPacket replaces the packet that is forwarded. The batch variants are called instead of the
// single-packet variants when the proxy reads packets in batches (Listen.EnableBatchReading).
type Handler interface {
	Start(Backend) error
	HandleClientPacket(*PacketContext)
	HandleServerPacket(*PacketContext)
	HandleClientBatch([]*PacketContext)
	HandleServerBatch([]*PacketContext)
	TransferBackend(Backend) error
	TransferFailed(string, error)
	Close() error
}

// ChunkRadiusProvider overrides the client-requested chunk radius used when a transfer asks the
// replacement backend to begin sending chunks.
type ChunkRadiusProvider interface {
	ChunkRadius() int32
}

// NopHandler forwards every packet and accepts every backend lifecycle event. Embed it in handlers
// that only need to override part of Handler.
type NopHandler struct{}

func (NopHandler) Start(Backend) error                { return nil }
func (NopHandler) HandleClientPacket(*PacketContext)  {}
func (NopHandler) HandleServerPacket(*PacketContext)  {}
func (NopHandler) HandleClientBatch([]*PacketContext) {}
func (NopHandler) HandleServerBatch([]*PacketContext) {}
func (NopHandler) TransferBackend(Backend) error      { return nil }
func (NopHandler) TransferFailed(string, error)       {}
func (NopHandler) Close() error                       { return nil }
