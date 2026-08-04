package proxy

import (
	"context"
	"log/slog"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Config configures a standalone Bedrock proxy.
type Config struct {
	LocalAddress  string
	RemoteAddress string
	Log           *slog.Logger
	Listen        minecraft.ListenConfig
	DialTimeout   time.Duration
	NewHandler    HandlerFactory

	// Dial may be set by advanced users to customise backend connections. When
	// Listen.EnableBatchReading is set, backend connections must be dialed with batch reading
	// enabled as well, as the proxy reads from them with ReadBatch.
	Dial DialFunc
}

// DialFunc establishes and logs into a backend, stopping before DoSpawn.
type DialFunc func(context.Context, string, login.IdentityData, login.ClientData, string) (Backend, error)

// Backend is the backend connection surface required by the native proxy.
type Backend interface {
	GameData() minecraft.GameData
	WritePacket(packet.Packet) error
	Close() error
	ReadPacket() (packet.Packet, error)
	ReadBatch() ([]packet.Packet, error)
	DoSpawn() error
	Flush() error
}

func defaultDial(timeout time.Duration, batchReading bool) DialFunc {
	return func(ctx context.Context, address string, identity login.IdentityData, client login.ClientData, _ string) (Backend, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return minecraft.Dialer{
			IdentityData:        identity,
			ClientData:          client,
			KeepXBLIdentityData: true,
			EnableBatchReading:  batchReading,
		}.DialContext(ctx, "raknet", address)
	}
}
